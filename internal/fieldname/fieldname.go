// Package fieldname reads and removes HTTP header fields by name
// case-insensitively (RFC 9110 §5.1).
//
// fasthttp's Peek, PeekAll and Del are byte-exact, which finds every line only
// while it canonicalizes the key; under DisableHeaderNormalizing the store keeps
// the spelling the peer sent, which for HTTP/2 and 3 is lower case.
//
// canonical says whether the caller knows the keys to be canonical — fasthttp
// keeps that private, so it is passed in; see headerlookup.Canonical.
package fieldname

import (
	"bytes"
	"iter"
	"slices"

	"github.com/gofiber/fiber/v3/internal/headerlist"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Peeker is the part of fasthttp's headers needed to read field lines by name.
type Peeker interface {
	Peek(key string) []byte
	PeekAll(key string) [][]byte
	PeekTrailerKeys() [][]byte
	All() iter.Seq2[[]byte, []byte]
	// VisitAll is deprecated in fasthttp in favor of All, but All builds an
	// iterator per call while this walks the store directly. Kept for the reads
	// that run per request and do not need to stop early.
	VisitAll(f func(key, value []byte))
}

// Deleter is the part of fasthttp's headers needed to remove a field by name.
type Deleter interface {
	Del(key string)
	PeekAll(key string) [][]byte
	All() iter.Seq2[[]byte, []byte]
}

// HasTrailers reports whether h holds a trailer section, received or announced.
// Peek and PeekAll fall through to it, so the keyed reads are taken only without one.
func HasTrailers(h Peeker) bool {
	return len(h.PeekTrailerKeys()) > 0
}

// MaxTrailers is the most trailer names DropTrailers takes on, since it walks the whole header for each.
const MaxTrailers = 16

// DropTrailers discards a request's trailer section and the names announced for it (RFC 9112 §7.1.2):
// fasthttp files a later Set or Add under such a name with that section, out of the header readers' reach.
// It reports false, leaving h as it is, for more than MaxTrailers names.
func DropTrailers(h *fasthttp.RequestHeader) bool {
	keys := h.PeekTrailerKeys()
	if len(keys) == 0 {
		return true
	}
	if len(keys) > MaxTrailers {
		return false
	}
	names := make([]string, len(keys))
	for i, key := range keys {
		names[i] = string(key)
	}
	// Del takes a name's header lines along with its trailer fields, so they are put back after.
	var kept []string
	for k, v := range h.All() {
		if slices.Contains(names, utils.UnsafeString(k)) {
			kept = append(kept, string(k), string(v))
		}
	}
	for _, name := range names {
		h.Del(name)
	}
	// With the trailer section empty, this has nothing to move into the header section.
	h.Del(fasthttp.HeaderTrailer)
	for i := 0; i < len(kept); i += 2 {
		h.Add(kept[i], kept[i+1])
	}
	return true
}

// ClearTrailers is DropTrailers in one pass for any number of names, putting each line back in the buffer a value
// read earlier aliases. It reports false, having dropped the section all the same, for more than MaxTrailers names.
func ClearTrailers(req *fasthttp.Request) bool {
	h := &req.Header
	keys := len(h.PeekTrailerKeys())
	if keys == 0 {
		return true
	}
	// Collected first, as the walk below collects them, so each line keeps its slot.
	h.Cookie("")
	tmp := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(tmp)
	saved := &tmp.Header
	h.CopyTo(saved)
	// Reset empties the received block, which the forwarding order is read from.
	if raw := h.RawHeaders(); len(raw) > 0 {
		block := slices.Clone(raw)
		SetReceived(req, &block)
	}

	// It returns the previous setting, which Reset clears.
	disabled := h.DisableNormalizing()
	h.Reset()
	if disabled {
		h.DisableNormalizing()
	}
	h.SetMethodBytes(saved.Method())
	h.SetRequestURIBytes(saved.RequestURI())
	h.SetProtocolBytes(saved.Protocol())
	for k, v := range saved.All() {
		switch name := utils.UnsafeString(k); {
		case utils.EqualFold(name, fasthttp.HeaderTrailer),
			utils.EqualFold(name, fasthttp.HeaderConnection) && utils.EqualFold(utils.UnsafeString(v), "close"):
			// The announced names, and the close flag, which holds no line of the store.
		case utils.EqualFold(name, fasthttp.HeaderConnection), utils.EqualFold(name, fasthttp.HeaderTransferEncoding):
			// As stored: Add drops the framing field and folds a second Connection line into the first.
			h.DisableSpecialHeader()
			h.AddBytesKV(k, v)
			h.EnableSpecialHeader()
		default:
			h.AddBytesKV(k, v)
		}
	}
	if saved.ConnectionClose() {
		h.SetConnectionClose()
	}
	if saved.ContentLength() == -1 {
		h.SetContentLength(-1)
	}
	return keys <= MaxTrailers
}

// InTrailer reports whether name is a field of a request's trailer section. A header line under such a name is not
// read: it may be the client's, with the Set meant to replace it filed with the section. A response's line still is.
func InTrailer(h *fasthttp.RequestHeader, name string) bool {
	for _, key := range h.PeekTrailerKeys() {
		if utils.EqualFold(utils.UnsafeString(key), name) {
			return true
		}
	}
	return false
}

type receivedKey struct{}

// SetReceived records block, field lines in the order they were received, for a request built in code from one
// received elsewhere, such as the net/http adaptor's: fasthttp keeps no received block for it.
func SetReceived(req *fasthttp.Request, block *[]byte) {
	req.SetUserValue(receivedKey{}, block)
}

// Received returns the field lines req was received with: those fasthttp read off the wire, or SetReceived's.
func Received(req *fasthttp.Request) []byte {
	if raw := req.Header.RawHeaders(); len(raw) > 0 {
		return raw
	}
	if block, ok := req.UserValue(receivedKey{}).(*[]byte); ok {
		return *block
	}
	return nil
}

// ReceivedOrder calls f with name's lines in the order raw received them, which a Del can lose in the store; a line
// raw lacks, such as one added in code, follows in stored order. Above MaxOrderedLines lines f gets raw's alone.
func ReceivedOrder(h *fasthttp.RequestHeader, raw []byte, name string, f func(value []byte)) {
	named := func(k []byte) bool { return utils.EqualFold(utils.UnsafeString(k), name) }
	var buf [8]orderedLine
	lines := buf[:0]
	over := false
	h.VisitAll(func(k, v []byte) {
		if over || !named(k) {
			return
		}
		if over = len(lines) == MaxOrderedLines; !over {
			lines = append(lines, orderedLine{name: k, value: v, rank: MaxOrderedLines + len(lines)})
		}
	})
	if over {
		visitReceived(h, raw, func(k, v []byte) {
			if named(k) {
				f(v)
			}
		})
		return
	}
	rankReceived(h, raw, lines, named)
	slices.SortStableFunc(lines, func(a, b orderedLine) int { return a.rank - b.rank })
	for _, l := range lines {
		f(l.value)
	}
}

// rawLine returns the first field line of the received header block raw, or nil at the end of the block, and the
// length it takes up. folded reports a line the next one continues (obs-fold), which only a copy can join.
func rawLine(raw []byte) (line []byte, n int, folded bool) { //nolint:nonamedreturns // gocritic unnamedResult asks for names
	for n < len(raw) {
		line = raw[n:]
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			line = line[:i]
			n += i + 1
		} else {
			n = len(raw)
		}
		if l := len(line); l > 0 && line[l-1] == '\r' {
			line = line[:l-1]
		}
		if len(line) == 0 {
			break
		}
		if line[0] != ' ' && line[0] != '\t' {
			return line, n, n < len(raw) && (raw[n] == ' ' || raw[n] == '\t')
		}
	}
	return nil, len(raw), false
}

// rawValue returns the value of line, trimmed as fasthttp stores it, if line is a field line named name. A request
// with whitespace before the colon never gets this far (RFC 9112 Section 5.1).
func rawValue(line []byte, name string) ([]byte, bool) {
	// The colon settles most lines before the name is compared.
	if len(line) <= len(name) || line[len(name)] != ':' || !utils.EqualFold(utils.UnsafeString(line[:len(name)]), name) {
		return nil, false
	}
	return headerlist.TrimOWS(line[len(name)+1:]), true
}

// JoinedInReceivedOrder reports whether combined joins name's lines in the received header block raw, in order.
// raw is read in place, so a folded line reports false.
func JoinedInReceivedOrder(raw []byte, name string, combined []byte) bool {
	found := false
	for {
		line, n, folded := rawLine(raw)
		if line == nil {
			return found && len(combined) == 0
		}
		raw = raw[n:]
		v, named := rawValue(line, name)
		if !named {
			continue
		}
		if folded {
			return false
		}
		if found {
			if len(combined) == 0 || combined[0] != ',' {
				return false
			}
			combined = combined[1:]
		}
		if !bytes.HasPrefix(combined, v) {
			return false
		}
		combined = combined[len(v):]
		found = true
	}
}

// MaxOrderedLines is the most lines KeepReceivedOrder and ReceivedOrder rank, since they match each received line
// against them.
const MaxOrderedLines = 64

// orderedLine is a stored line KeepReceivedOrder may move. rank is its place among the received lines, or
// MaxOrderedLines plus its stored place for one that was not received, which so stays behind them in stored order.
type orderedLine struct {
	name, value []byte
	group, rank int
}

// KeepReceivedOrder puts the lines of each name match accepts that is stored on several lines back in the order
// they were received, which the store loses whenever fasthttp moves its last line into a removed one's slot, as
// it does for Transfer-Encoding after an empty chunked body. A line added or changed in code follows them.
// It reports false, leaving h as it is, for more than MaxOrderedLines such lines. raw is the block from Received.
func KeepReceivedOrder(h *fasthttp.RequestHeader, raw []byte, match func(name []byte) bool) bool {
	var buf [8]orderedLine
	lines := buf[:0]
	over := false
	h.VisitAll(func(k, v []byte) {
		if !match(k) || over {
			return
		}
		if over = len(lines) == MaxOrderedLines; !over {
			lines = append(lines, orderedLine{name: k, value: v, group: len(lines), rank: MaxOrderedLines + len(lines)})
		}
	})
	if over {
		return false
	}
	repeated := false
	for i := range lines {
		for j := range i {
			if utils.EqualFold(utils.UnsafeString(lines[j].name), utils.UnsafeString(lines[i].name)) {
				lines[i].group, repeated = lines[j].group, true
				break
			}
		}
	}
	if !repeated || len(raw) == 0 {
		return true
	}

	rankReceived(h, raw, lines, match)
	for g := range lines {
		if lines[g].group == g && !rankedInOrder(lines, g) {
			rewriteInRankOrder(h, lines)
			break
		}
	}
	return true
}

// rankReceived ranks the stored lines by the received lines they equal, walking the received block once.
func rankReceived(h *fasthttp.RequestHeader, raw []byte, lines []orderedLine, match func(name []byte) bool) {
	rank := 0
	visitReceived(h, raw, func(k, v []byte) {
		if !match(k) {
			return
		}
		for i := range lines {
			l := &lines[i]
			// Trimmed: a value net/http read off HTTP/2 keeps the edge whitespace its received line drops.
			if l.rank >= MaxOrderedLines && bytes.Equal(headerlist.TrimOWS(l.value), v) &&
				utils.EqualFold(utils.UnsafeString(l.name), utils.UnsafeString(k)) {
				l.rank = rank
				rank++
				return
			}
		}
	})
}

// visitReceived calls visit with each field line of the received block raw, in order.
func visitReceived(h *fasthttp.RequestHeader, raw []byte, visit func(k, v []byte)) {
	// A folded line (obs-fold) only AllInOrder joins, and it does so in place, so on a copy.
	if bytes.Contains(raw, []byte("\n ")) || bytes.Contains(raw, []byte("\n\t")) {
		scratch := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(scratch)
		h.CopyTo(&scratch.Header)
		for k, v := range scratch.Header.AllInOrder() {
			visit(k, v)
		}
		return
	}
	for {
		line, n, _ := rawLine(raw)
		if line == nil {
			return
		}
		raw = raw[n:]
		if colon := bytes.IndexByte(line, ':'); colon > 0 {
			visit(line[:colon], headerlist.TrimOWS(line[colon+1:]))
		}
	}
}

func rankedInOrder(lines []orderedLine, group int) bool {
	last := -1
	for _, l := range lines[group:] {
		if l.group == group {
			if l.rank < last {
				return false
			}
			last = l.rank
		}
	}
	return true
}

// rewriteInRankOrder writes every name of lines anew, in rank order: each Del can move another name's lines.
func rewriteInRankOrder(h *fasthttp.RequestHeader, lines []orderedLine) {
	// Copied into one buffer, since the Adds below reuse the buffers Del frees.
	type copied struct{ rank, name, value, end int }
	var (
		space [512]byte
		buf   [8]copied
	)
	arena, ordered := space[:0], buf[:0]
	for _, l := range lines {
		c := copied{rank: l.rank, name: len(arena)}
		arena = append(arena, lines[l.group].name...)
		c.value = len(arena)
		arena = append(arena, l.value...)
		c.end = len(arena)
		ordered = append(ordered, c)
	}
	slices.SortStableFunc(ordered, func(a, b copied) int { return a.rank - b.rank })

	for g, l := range lines {
		if l.group != g {
			continue
		}
		// One canonical spelling lets the byte-exact Del find every line, in either store.
		canonical := IsCanonical(utils.UnsafeString(l.name))
		for _, m := range lines[g+1:] {
			canonical = canonical && (m.group != g || bytes.Equal(m.name, l.name))
		}
		Del(h, utils.UnsafeString(l.name), canonical)
	}
	for _, c := range ordered {
		h.AddBytesKV(arena[c.name:c.value], arena[c.value:c.end])
	}
}

// Lines returns every field line stored under name. The walk replaces the
// byte-exact lookup rather than backing it up: a message can carry both
// spellings, and an empty canonical line would hide the value beside it.
// A name a request holds in its trailer section has no lines (see InTrailer); a response's header lines are read.
//
//nolint:revive // flag-parameter: canonical is a property of the header store, not a mode of operation
func Lines(h Peeker, name string, canonical bool) [][]byte {
	if canonical && !HasTrailers(h) {
		// PeekAll hands back the header's own slice without allocating.
		return h.PeekAll(name)
	}
	if req, ok := h.(*fasthttp.RequestHeader); ok && InTrailer(req, name) {
		return nil
	}

	// Walked rather than ranged: this runs per key header on every cacheable
	// request, and All's iterator costs more than the match itself does.
	var values [][]byte
	h.VisitAll(func(k, v []byte) {
		if utils.EqualFold(utils.UnsafeString(k), name) {
			values = append(values, v)
		}
	})
	return values
}

// First returns the first field line stored under name that holds anything, or
// nil if there is none. The names fasthttp keeps in a slot of their own are
// unaffected either way: every spelling is routed into the slot on the way in.
//
// Empty lines are stepped over rather than answered with, because a message can
// carry the name more than once and Peek reports the first line whether or not
// it holds a value: an empty "Cache-Control:" ahead of "Cache-Control: private"
// read as a response saying nothing about caching. A field line that is present
// and empty says nothing a caller can act on, so nothing is lost by looking past
// it for one that does.
//
// Repetition itself is left to the caller to judge, because what it means
// depends on the field: a response may carry Cache-Control twice and mean the
// list, while a second Origin or Authorization line is malformed and cannot be
// resolved by picking one. headerlookup.Value refuses that case for the request
// fields where it arises.
//
// A name a request holds in its trailer section reads as absent (see InTrailer); a response's header line is read.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func First(h Peeker, name string, canonical bool) []byte {
	if canonical && !HasTrailers(h) {
		v := Peek(h, name)
		if len(v) > 0 {
			return v
		}
		if v == nil {
			// Nothing is stored under the name, so there is nothing beside it
			// to find and the walk below is skipped: a nil answer means absent,
			// a non-nil empty one means a line that is present and empty. That
			// distinction is fasthttp's to keep, so Test_First_EmptyLineIsNotNil
			// pins it — if it ever stops holding, that test says so rather than
			// this quietly reverting to reading only the first line.
			return nil
		}

		// Reached only for a field that is present and empty, which is the shape
		// being guarded against rather than one anything sends by accident.
		// PeekAll costs no allocation and resolves the slotted names.
		for _, v := range h.PeekAll(name) {
			if len(v) > 0 {
				return v
			}
		}
		return nil
	}

	// Split out rather than written below: All's iterator is heap-allocated
	// where the compiler cannot see the branch is dead, so holding it in this
	// function cost the canonical path an allocation it never used. The request
	// trailer check sits there too: inlined here, it slowed the canonical path.
	return firstFold(h, name)
}

// firstFold answers First for a store whose field names are spelled however the
// peer sent them, or that holds a trailer section.
func firstFold(h Peeker, name string) []byte {
	if req, ok := h.(*fasthttp.RequestHeader); ok && InTrailer(req, name) {
		return nil
	}
	// Walked rather than ranged, for the reason Lines gives: All builds a
	// heap-allocated iterator per call. Scanning past the match costs less than
	// the early exit ranging would buy.
	var found []byte
	h.VisitAll(func(k, v []byte) {
		if found == nil && len(v) > 0 && utils.EqualFold(utils.UnsafeString(k), name) {
			found = v
		}
	})
	return found
}

// Canonical reports whether every field name in h is spelled the way fasthttp
// normalizes it, so the byte-exact Peek and Del find them all.
//
// Ask this of a response rather than reading the app config: a proxied response
// is parsed by an outbound fasthttp.Client carrying its own normalizing setting,
// so a default-normalizing app can hold a response of lower-case names. It is
// one walk, against one per field for the case-insensitive reads it saves.
func Canonical(h *fasthttp.ResponseHeader) bool {
	canonical := true
	// Walked rather than ranged, for the reason Lines gives: All allocates an
	// iterator per call, and this runs per response. The deprecation is excluded
	// in .golangci.yml rather than by an inline directive, which the lint cache
	// leaves reported as unused on some runs.
	h.VisitAll(func(k, _ []byte) {
		if canonical && !isCanonicalName(k) {
			canonical = false
		}
	})
	return canonical
}

// isCanonicalName reports whether name is upper case at the start of each
// "-"-separated token and lower case everywhere else, which is what fasthttp's
// normalization produces.
func isCanonicalName(name []byte) bool {
	upper := true
	for _, c := range name {
		if upper {
			if c >= 'a' && c <= 'z' {
				return false
			}
		} else if c >= 'A' && c <= 'Z' {
			return false
		}
		upper = c == '-'
	}
	return true
}

// Del removes every field line named name, whatever case it is stored under.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func Del(h Deleter, name string, canonical bool) {
	h.Del(name)
	if canonical {
		// The scan below would only confirm Del found everything, at the cost
		// of an iterator allocation per name on an otherwise alloc-free path.
		return
	}

	// Collected before deleting, like DelOthers, rather than rescanning after
	// each removal: ResponseHeader.All reports a default Content-Type whenever
	// the slot is empty, so a rescan would keep finding the name it just
	// deleted. A name stored non-canonically in a normalizing store is the same
	// shape, since Del normalizes the key before matching and so cannot remove
	// it at all.
	var others []string
	for k := range h.All() {
		if utils.EqualFold(utils.UnsafeString(k), name) {
			others = append(others, string(k))
		}
	}

	for _, k := range others {
		h.Del(k)
	}
}

// DelOthers removes every stored spelling of name except name itself, which the
// caller is about to write or delete under that key. Every other one, not just
// the first: stopping early leaves the rest beside the value the caller writes.
func DelOthers(h Deleter, name string) {
	// Collected before deleting: removing while ranging over the store is not safe.
	var others []string
	for k := range h.All() {
		if ks := utils.UnsafeString(k); ks != name && utils.EqualFold(ks, name) {
			others = append(others, string(k))
		}
	}

	for _, k := range others {
		h.Del(k)
	}
}

// DelSet removes every field line matching any of names in a single pass, which
// beats Del's per-name scan when the whole set is known up front.
func DelSet(h Deleter, names []string) {
	var removed []string
	for k := range h.All() {
		if ContainsFold(names, utils.UnsafeString(k)) {
			removed = append(removed, string(k))
		}
	}

	// A repeated name is deleted twice; Del removes every line at once, so
	// scanning to avoid that costs more than making the call.
	for _, name := range removed {
		h.Del(name)
	}
}

// ContainsFold reports whether needle equals any of haystack, case-insensitively.
func ContainsFold(haystack []string, needle string) bool {
	for _, h := range haystack {
		if utils.EqualFold(h, needle) {
			return true
		}
	}
	return false
}

// Byte classes of a field name, for IsCanonical.
const (
	keyOther   byte = iota // a token byte that is not a letter
	keyLower               // a-z
	keyUpper               // A-Z
	keyInvalid             // not a token byte (RFC 9110 Section 5.6.2)
)

// keyClass classifies every byte a field name can hold. The token set is the
// one fasthttp normalizes; any other byte makes it store the name as sent.
var keyClass = func() [256]byte {
	var t [256]byte
	for i := range t {
		t[i] = keyInvalid
	}
	for _, c := range []byte("!#$%&'*+-.^_`|~0123456789") {
		t[c] = keyOther
	}
	for c := byte('a'); c <= 'z'; c++ {
		t[c] = keyLower
		t[c-'a'+'A'] = keyUpper
	}
	return t
}()

// IsCanonical reports whether name is already in fasthttp's canonical form: a
// token with an upper-case letter first and after each '-' and lower case
// elsewhere. fasthttp derives that form again on every keyed call, three
// passes and a copy, only to arrive at the same bytes; a name that already has
// it can take the *Canonical methods instead, which store and find exactly what
// the normalizing ones would whether or not the store normalizes.
// Test_IsCanonical_MatchesFasthttp keeps the form in step.
func IsCanonical(name string) bool {
	n := len(name)
	if n == 0 {
		return false
	}
	// Names such as X-Request-ID fail only at their last byte, so the tail is
	// checked first: an upper-case letter not following a '-' cannot end a
	// canonical name.
	if n > 1 && name[n-1] >= 'A' && name[n-1] <= 'Z' && name[n-2] != '-' {
		return false
	}
	upper := true
	for i := range n {
		c := name[i]
		switch keyClass[c] {
		case keyInvalid:
			return false
		case keyLower:
			if upper {
				return false
			}
		case keyUpper:
			if !upper {
				return false
			}
		}
		upper = c == '-'
	}
	return true
}

// Peek is fasthttp's byte-exact Peek, minus the key normalization when name is
// already canonical; Test_Peek_MatchesFasthttp keeps the two answering alike in
// both store modes.
func Peek(h Peeker, name string) []byte {
	if IsCanonical(name) {
		// Concrete calls only: through the interface the key would escape.
		switch h := h.(type) {
		case *fasthttp.RequestHeader:
			return h.PeekCanonical(utils.UnsafeBytes(name))
		case *fasthttp.ResponseHeader:
			return h.PeekCanonical(utils.UnsafeBytes(name))
		}
	}
	return h.Peek(name)
}
