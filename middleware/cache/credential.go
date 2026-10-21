package cache

import (
	"container/list"
	"context"
	"encoding/binary"
	"errors"
	"hash/maphash"
	"math"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/headerlist"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
)

// credentialHeaders carry nothing but a credential.
var credentialHeaders = []string{fiber.HeaderAuthorization, fiber.HeaderCookie, fiber.HeaderProxyAuthorization, "X-API-Key", "X-Auth-Token", "X-Access-Token"}

func (cfg *Config) carriesCredential(name string) bool {
	return fieldname.ContainsFold(credentialHeaders, name) || fieldname.ContainsFold(cfg.KeyCredentialHeaders, name)
}

func (cfg *Config) appendCredentials(dst, names []string) []string {
	for _, name := range names {
		if cfg.carriesCredential(name) {
			dst = append(dst, name)
		}
	}
	return dst
}

// varyListsCredential reports whether a Vary value names a credential header, without allocating.
func (cfg *Config) varyListsCredential(vary []byte) bool {
	for name := range headerlist.All(utils.UnsafeString(vary)) {
		if cfg.carriesCredential(name) {
			return true
		}
	}
	return false
}

// holdsCredentialResponse reports whether the entry found can hold one client's response.
func (cfg *Config) holdsCredentialResponse(c fiber.Ctx, vary []byte) bool {
	// Config first, as it is cheaper than Vary or the headers; a custom key is not taken to hold a declared header.
	if len(cfg.KeyCookies) > 0 || cfg.keyHeadersNameCredential || (cfg.usesDefaultKeyGenerator && len(cfg.KeyCredentialHeaders) > 0) {
		return true
	}
	return cfg.varyListsCredential(vary) || (!cfg.usesDefaultKeyGenerator && hasHeaderValue(c, cfg.KeyCredentialHeaders))
}

// refusesCredential reports whether status is the origin refusing the credential (407 for a proxy's).
func refusesCredential(status int) bool {
	return status == fiber.StatusUnauthorized ||
		status == fiber.StatusForbidden ||
		status == fiber.StatusProxyAuthRequired
}

// acceptsCredential reports whether status is the origin accepting the credential again: a 2xx or a
// 304. Not a redirect, which is how an app sends an ended session to its login page.
func acceptsCredential(status int) bool {
	return (status >= fiber.StatusOK && status < fiber.StatusMultipleChoices) || status == fiber.StatusNotModified
}

// errorRefusesCredential is refusesCredential for a returned *fiber.Error, wrapped or not.
func errorRefusesCredential(err error) bool {
	var fiberErr *fiber.Error
	// A nil *fiber.Error held in an error matches errors.As and has no Code.
	return errors.As(err, &fiberErr) && fiberErr != nil && refusesCredential(fiberErr.Code)
}

// credentialScope is how far a refusal reaches.
type credentialScope struct {
	entry bool // the refusal ends the entry the request found
	keyed bool // the key holds the credential, so the Vary manifest is its own too
}

// requestCredentials is what the request sends that ties an entry to its credential, found once.
type requestCredentials struct {
	authorization bool // an Authorization header, which is in every key
	keyed         bool // Authorization, or a credential the default key holds
	selected      bool // a credential header the entry's Vary names or, with a custom key, a declared one
}

// credentialsOf finds what the request sends, before the handler can clear it.
func (cfg *Config) credentialsOf(c fiber.Ctx, varyNames []string, hasAuthorization bool) requestCredentials { //nolint:revive // flag-parameter: a property of the request, not a mode of operation
	var named [defaultVaryNames]string
	credentials := cfg.appendCredentials(named[:0], varyNames)
	if !cfg.usesDefaultKeyGenerator {
		// A custom key is opaque: the declared headers are taken to be in it.
		credentials = append(credentials, cfg.KeyCredentialHeaders...)
	}
	return requestCredentials{
		authorization: hasAuthorization,
		keyed:         hasAuthorization || cfg.keyHoldsRequestCredential(c),
		selected:      len(credentials) > 0 && hasHeaderValue(c, credentials),
	}
}

// scope says how far a refusal reaches: the entry ends only if the key holds the credential or the Vary selects it.
func (creds requestCredentials) scope() credentialScope {
	return credentialScope{keyed: creds.keyed, entry: creds.keyed || creds.selected}
}

// mayKeyCredential reports whether credentialsOf can find a credential at all, so a hit that cannot
// have a record looks for none.
func (cfg *Config) mayKeyCredential(hasAuthorization bool, varyNames []string) bool { //nolint:revive // flag-parameter: a property of the request, not a mode of operation
	return hasAuthorization || len(cfg.KeyCookies) > 0 || len(cfg.credentialKeyHeaders) > 0 ||
		slices.ContainsFunc(varyNames, cfg.carriesCredential)
}

const (
	minRefusalRecords       = 1024  // the fewest records an in-memory store keeps
	unlimitedRefusalRecords = 16384 // how many it keeps when MaxBytes is 0
	refusalRecordSize       = 256   // bytes of MaxBytes one record stands for
	droppedPerRecord        = 4     // dropped records remembered for each one kept
	refusalGroups           = 64    // groups of URLs, each with a marker of its own
)

// markerPoll is how long a record another instance wrote can go unseen.
const markerPoll = 250 * time.Millisecond

// refusalMarker tells the instances sharing a Storage that a record may exist, so hits look only then:
// one marker covers every URL and one each group. The records are keyed under it too, where no entry key lands.
const refusalMarker = cacheKeyVersion + "|refused"

var groupMarkers = func() [refusalGroups]string {
	var markers [refusalGroups]string
	for i := range markers {
		markers[i] = refusalMarker + "|" + strconv.Itoa(i)
	}
	return markers
}()

// groupOf puts a URL, or a custom key, in the group of a marker. Every instance does it alike.
func groupOf(s string) int {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 16777619
	}
	return int(h % refusalGroups)
}

// refusalLimit is how many refusal records an in-memory store keeps.
func (cfg *Config) refusalLimit() int {
	if cfg.refusalRecordLimit > 0 {
		return cfg.refusalRecordLimit
	}
	if cfg.MaxBytes == 0 {
		return unlimitedRefusalRecords
	}
	return int(min(max(cfg.MaxBytes/refusalRecordSize, minRefusalRecords), math.MaxInt32))
}

// recordGate says whether refusal records may exist, for every URL or for one group of them.
type recordGate struct {
	until atomic.Uint64 // the second no record written here outlives
	seen  atomic.Int64  // the millisecond its marker was last read
	open  atomic.Bool   // whether the marker was there
}

// refusalRecords keeps the records of refused credentials: bounded in memory, left to expire in a Storage.
type refusalRecords struct {
	cfg     *Config
	m       *manager
	keys    keyLog
	dropped droppedRecords
	family  Config // cfg keyed by the credential headers alone, so a record reaches every variant
	gates   [refusalGroups]recordGate
	expiry  [refusalGroups]atomic.Uint64 // the second the longest-lived credential entry stored in the group stops being kept
	all     recordGate
	poll    int64 // milliseconds between reads of a marker
}

func newRefusalRecords(cfg *Config, m *manager) *refusalRecords {
	r := &refusalRecords{cfg: cfg, family: *cfg, m: m, poll: markerPoll.Milliseconds()}
	r.family.KeyHeaders = cfg.credentialKeyHeaders
	limit := cfg.refusalLimit()
	if cfg.Storage == nil {
		r.keys.limit = limit
	}
	r.dropped = newDroppedRecords(limit * droppedPerRecord)
	return r
}

// key is where the refusal is recorded, under refusalMarker: the key without its other dimensions, so
// one record reaches every variant, or, for a credential the key does not hold, a digest of the
// credential headers the Vary selects by; "" for none. A HEAD is recorded as the GET it mirrors.
func (r *refusalRecords) key(c fiber.Ctx, generated string, varyNames []string, creds requestCredentials) string {
	bufPtr := acquireKeyBuffer()
	buf := r.appendKey((*bufPtr)[:0], c, generated, varyNames, creds)
	key := string(buf)
	releaseKeyBuffer(bufPtr, buf)
	return key
}

// refusedFor reports whether the request's credential is recorded as refused. The key is built in a
// pooled buffer, which the in-memory store can look up as it is: only a Storage gets a string.
func (r *refusalRecords) refusedFor(ctx context.Context, c fiber.Ctx, generated string, varyNames []string, creds requestCredentials) bool {
	bufPtr := acquireKeyBuffer()
	buf := r.appendKey((*bufPtr)[:0], c, generated, varyNames, creds)
	refused := false
	if len(buf) > 0 {
		key := utils.UnsafeString(buf)
		if r.cfg.Storage != nil {
			key = string(buf)
		}
		refused = r.refused(ctx, key)
	}
	releaseKeyBuffer(bufPtr, buf)
	return refused
}

// appendKey appends the key of the refusal record to buf; see key.
func (r *refusalRecords) appendKey(buf []byte, c fiber.Ctx, generated string, varyNames []string, creds requestCredentials) []byte {
	cfg := r.cfg
	// keyed covers Authorization, which is in every key, a custom one too.
	if !creds.selected && !creds.keyed {
		return buf
	}

	// A credential the key holds is recorded under the key alone: its variants need not vary alike.
	var named [defaultVaryNames]string
	var varied []string
	if creds.selected && !creds.keyed {
		varied = cfg.appendCredentials(named[:0], varyNames)
		if !cfg.usesDefaultKeyGenerator {
			varied = append(varied, cfg.KeyCredentialHeaders...)
			slices.Sort(varied)
			varied = slices.Compact(varied)
		}
	}

	canonical := headerlookup.Canonical(c)
	buf = append(buf, refusalMarker...)
	buf = append(buf, '|')
	if method := c.Method(); method == fiber.MethodHead {
		buf = append(buf, fiber.MethodGet...)
	} else {
		buf = append(buf, method...)
	}
	buf = append(buf, '|')
	if cfg.usesDefaultKeyGenerator {
		buf = appendDefaultKey(buf, c, &r.family)
		if creds.authorization {
			buf = append(buf, "|auth="...)
			buf = appendAuthHash(buf, fieldname.Lines(&c.Request().Header, fiber.HeaderAuthorization, canonical))
		}
	} else {
		buf = append(buf, generated...)
	}
	if len(varied) > 0 {
		buf = appendVaryKey(buf, varied, &c.Request().Header, canonical)
	}
	return buf
}

// group is the group of URLs the request belongs to: its path, or its key when that is a custom one.
func (r *refusalRecords) group(c fiber.Ctx, generated string) int {
	if r.cfg.usesDefaultKeyGenerator {
		return groupOf(c.Path())
	}
	return groupOf(generated)
}

// mayHold reports whether a record can exist for a hit on this URL to find: one written here or, in
// an external Storage, one another instance announced.
func (r *refusalRecords) mayHold(ctx context.Context, c fiber.Ctx, generated string) bool {
	now := uint64(utils.Timestamp())
	if now >= r.all.until.Load() && !r.announced(ctx, &r.all, refusalMarker) {
		return false
	}
	group := r.group(c, generated)
	gate := &r.gates[group]
	return now < gate.until.Load() || r.announced(ctx, gate, groupMarkers[group])
}

// announced reports whether a record was announced under the marker, which it reads at most once
// per markerPoll. A Storage that cannot answer counts as an announcement.
func (r *refusalRecords) announced(ctx context.Context, gate *recordGate, marker string) bool {
	if r.cfg.Storage == nil {
		return false
	}
	nowMilli := time.Now().UnixMilli()
	seen := gate.seen.Load()
	// since is negative once the clock is set back: poll then, rather than when it catches up.
	if since := nowMilli - seen; (since >= 0 && since < r.poll) || !gate.seen.CompareAndSwap(seen, nowMilli) {
		return gate.open.Load()
	}
	raw, err := r.m.getRaw(ctx, marker)
	if err != nil && !errors.Is(err, errCacheMiss) {
		log.Warnf("cache: failed to read the marker of refusal records: %v", err)
	}
	open := !errors.Is(err, errCacheMiss)
	raise(&gate.until, lapseOf(raw)) // hits look until then without polling again
	gate.open.Store(open)
	return open
}

// refused reports whether a refusal was recorded under key. A storage that cannot answer counts as
// one, and so does a key whose record was dropped live or could not be written.
func (r *refusalRecords) refused(ctx context.Context, key string) bool {
	if r.dropped.has(key, uint64(utils.Timestamp())) {
		return true
	}
	_, err := r.m.getRaw(ctx, key)
	if err == nil {
		return true
	}
	if errors.Is(err, errCacheMiss) {
		return false
	}
	log.Warnf("cache: failed to read the record of a refused credential: %v", err)
	return true
}

// record keeps the refusal under key, in the group of URLs the lookup saw, for Expiration, until entryExp
// (unix seconds) or until the group's longest-lived credential entry stops being kept, whichever is
// latest, and never shorter than the record there already. A record the Storage cannot take is remembered here.
func (r *refusalRecords) record(ctx context.Context, group int, key string, entryExp uint64) {
	retention := r.cfg.Expiration
	clock := safeUnixSeconds(r.cfg.now())
	if last := max(entryExp, r.expiry[group].Load()); last > clock {
		retention = max(retention, secondsToDuration(last-clock))
	}
	now := uint64(utils.Timestamp())
	until := max(now+uint64(retention.Seconds()), r.lapse(ctx, key))
	// Raised before the write; the 2 covers the store's rounding and a clock tick.
	r.open(group, until+2)
	if err := r.m.setRaw(ctx, key, lapseValue(until), secondsToDuration(until-now)); err != nil {
		log.Warnf("cache: failed to record the refusal of a credential: %v", err)
		r.dropped.add(key, until+2)
		return
	}
	// The store counts from the write, which may have taken a while.
	end := uint64(utils.Timestamp()) + (until - now) + 2
	r.open(group, end)
	if r.cfg.Storage != nil {
		r.announce(ctx, &r.all, refusalMarker)
		r.announce(ctx, &r.gates[group], groupMarkers[group])
	}
	if oldest := r.keys.add(key, end); oldest.key != "" {
		// Dropped to bound the store: while it could still have applied, hits for it fail closed.
		if oldest.until > now {
			r.dropped.add(oldest.key, oldest.until)
		}
		if err := r.m.del(ctx, oldest.key); err != nil {
			log.Warnf("cache: failed to drop the oldest record of a refused credential: %v", err)
		}
	}
}

// open has hits look for a record of the group until the second given.
func (r *refusalRecords) open(group int, until uint64) {
	raise(&r.all.until, until)
	raise(&r.gates[group].until, until)
}

// noteLifetime takes in how long a credential's entry of the group is kept, so a record outlives it.
func (r *refusalRecords) noteLifetime(group int, lifetime time.Duration) {
	if lifetime > 0 {
		raise(&r.expiry[group], safeUnixSeconds(r.cfg.now())+uint64(lifetime.Seconds()))
	}
}

// announce marks the gate in the Storage until its second, or the later one the marker already says.
func (r *refusalRecords) announce(ctx context.Context, gate *recordGate, marker string) {
	raise(&gate.until, r.lapse(ctx, marker))
	until := gate.until.Load()
	now := uint64(utils.Timestamp())
	if until <= now {
		return // a TTL of zero would mean no expiry
	}
	if err := r.m.setRaw(ctx, marker, lapseValue(until), secondsToDuration(until-now)); err != nil {
		log.Warnf("cache: failed to announce the record of a refused credential: %v", err)
	}
}

// lapse is the second the record or marker under key lapses, as its value says; 0 for none.
func (r *refusalRecords) lapse(ctx context.Context, key string) uint64 {
	raw, err := r.m.getRaw(ctx, key)
	if err != nil {
		if !errors.Is(err, errCacheMiss) {
			log.Warnf("cache: failed to read a record of a refused credential: %v", err)
		}
		return 0
	}
	return lapseOf(raw)
}

// lapseOf reads the second a record or marker lapses off its value; 0 when it does not say.
func lapseOf(raw []byte) uint64 {
	if len(raw) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(raw)
}

func lapseValue(until uint64) []byte {
	return binary.BigEndian.AppendUint64(nil, until)
}

// raise lifts a to v if it is lower.
func raise(a *atomic.Uint64, v uint64) {
	for old := a.Load(); v > old; old = a.Load() {
		if a.CompareAndSwap(old, v) {
			return
		}
	}
}

// forget drops the record once the origin accepts the credential again.
func (r *refusalRecords) forget(ctx context.Context, key string) {
	r.keys.remove(key)
	r.dropped.remove(key)
	if err := r.m.del(ctx, key); err != nil {
		log.Warnf("cache: failed to clear the record of a refused credential: %v", err)
	}
}

// loggedKey is a key and the second it lapses.
type loggedKey struct {
	key   string
	until uint64
}

// keyLog remembers up to limit keys, forgetting the oldest first; with no limit it keeps none.
type keyLog struct {
	items map[string]*list.Element
	order list.List
	mu    sync.Mutex
	limit int
}

// add remembers key until the second given, and returns the oldest key forgotten to stay in the limit.
func (l *keyLog) add(key string, until uint64) loggedKey {
	if l.limit == 0 {
		return loggedKey{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if element, ok := l.items[key]; ok {
		if logged, ok := element.Value.(loggedKey); ok {
			element.Value = loggedKey{key: key, until: max(logged.until, until)}
		}
		l.order.MoveToBack(element)
		return loggedKey{}
	}
	if l.items == nil {
		l.items = make(map[string]*list.Element)
	}
	l.items[key] = l.order.PushBack(loggedKey{key: key, until: until})
	if l.order.Len() <= l.limit {
		return loggedKey{}
	}
	oldest, ok := l.order.Remove(l.order.Front()).(loggedKey)
	if !ok {
		return loggedKey{}
	}
	delete(l.items, oldest.key)
	return oldest
}

func (l *keyLog) remove(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if element, ok := l.items[key]; ok {
		l.order.Remove(element)
		delete(l.items, key)
	}
}

// droppedRecords remembers, by hash and in a fraction of the room, the records dropped while they could
// still apply, so those credentials still fail closed. Once full it forgets the oldest, whose variants
// are served until they expire.
type droppedRecords struct {
	index map[uint64]int // hash -> slot
	ring  []droppedRecord
	seed  maphash.Seed
	mu    sync.Mutex
	next  int // the slot taken over once the ring is full
	limit int
}

type droppedRecord struct {
	hash  uint64
	until uint64
}

func newDroppedRecords(limit int) droppedRecords {
	return droppedRecords{limit: limit, seed: maphash.MakeSeed()}
}

// add remembers key until the second given, in the place of the oldest once the ring is full.
func (d *droppedRecords) add(key string, until uint64) {
	if d.limit == 0 {
		return
	}
	hash := maphash.String(d.seed, key)
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.index == nil {
		d.index = make(map[uint64]int)
	}
	if slot, ok := d.index[hash]; ok {
		d.ring[slot].until = max(d.ring[slot].until, until)
		return
	}
	slot := len(d.ring)
	if slot < d.limit {
		d.ring = append(d.ring, droppedRecord{})
	} else {
		slot = d.next
		d.next = (d.next + 1) % d.limit
		if indexed, ok := d.index[d.ring[slot].hash]; ok && indexed == slot {
			delete(d.index, d.ring[slot].hash)
		}
	}
	d.ring[slot] = droppedRecord{hash: hash, until: until}
	d.index[hash] = slot
}

// has reports whether key was dropped while it could still apply.
func (d *droppedRecords) has(key string, now uint64) bool {
	if d.limit == 0 {
		return false
	}
	hash := maphash.String(d.seed, key)
	d.mu.Lock()
	defer d.mu.Unlock()

	slot, ok := d.index[hash]
	return ok && d.ring[slot].until > now
}

func (d *droppedRecords) remove(key string) {
	if d.limit == 0 {
		return
	}
	hash := maphash.String(d.seed, key)
	d.mu.Lock()
	defer d.mu.Unlock()

	if slot, ok := d.index[hash]; ok {
		d.ring[slot] = droppedRecord{}
		delete(d.index, hash)
	}
}

// keyHoldsRequestCredential reports whether the default key holds a credential the request carries.
func (cfg *Config) keyHoldsRequestCredential(c fiber.Ctx) bool {
	return cfg.usesDefaultKeyGenerator && sendsCredential(c, cfg.KeyCookies, cfg.credentialKeyHeaders)
}

// sendsCredential reports whether the request sends one of the cookies or a value for any of the headers.
func sendsCredential(c fiber.Ctx, cookies, headers []string) bool {
	for _, name := range cookies {
		if c.Cookies(name) != "" {
			return true
		}
	}
	return hasHeaderValue(c, headers)
}

// hasHeaderValue reports whether the request sends a value for any of the headers.
func hasHeaderValue(c fiber.Ctx, names []string) bool {
	canonical := headerlookup.Canonical(c)
	for _, name := range names {
		if utils.EqualFold(name, fiber.HeaderCookie) {
			// Walk fasthttp's cookie store: building the Cookie line allocates.
			for range c.Request().Header.Cookies() {
				return true
			}
			continue
		}
		if len(fieldname.First(&c.Request().Header, name, canonical)) > 0 {
			return true
		}
	}
	return false
}
