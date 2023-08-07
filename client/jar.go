package client

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

var endOfTime = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

type jar struct {
	mu sync.Mutex

	entries map[string]map[string]*entry

	nextSeqNum uint64
}

type entry struct {
	Key        []byte
	Value      []byte
	Domain     []byte
	Path       []byte
	SameSite   fasthttp.CookieSameSite
	Secure     bool
	HttpOnly   bool
	Persistent bool
	Expires    time.Time
	Creation   time.Time
	LastAccess time.Time

	seqNum uint64
}

func (e *entry) id() string {
	return fmt.Sprintf("%s:%s:%s", utils.UnsafeString(e.Domain), utils.UnsafeString(e.Path), utils.UnsafeString(e.Key))
}
func (e *entry) shouldSend(https bool, host, path []byte) bool {
	return e.domainMatch(host) && e.pathMatch(path) && (https || !e.Secure)
}

func (e *entry) domainMatch(host []byte) bool {
	if utils.EqualFold(e.Domain, host) {
		return true
	}

	return hasDotSuffix(host, e.Domain)
}

func (e *entry) pathMatch(path []byte) bool {
	if utils.EqualFold(e.Path, path) {
		return true
	}
	if strings.HasPrefix(utils.UnsafeString(path), utils.UnsafeString(e.Path)) {
		if e.Path[len(e.Path)-1] == '/' {
			return true // The "/any/" matches "/any/path" case.
		} else if path[len(e.Path)] == '/' {
			return true // The "/any" matches "/any/path" case.
		}
	}
	return false
}

func (e *entry) reset() {
	e.Key = []byte{}
	e.Value = []byte{}
	e.Domain = []byte{}
	e.Path = []byte{}
	e.SameSite = fasthttp.CookieSameSiteDefaultMode
	e.Secure = true
	e.HttpOnly = true
	e.Persistent = false

	now := time.Now()
	e.Expires = now
	e.Creation = now
	e.LastAccess = now

	e.seqNum = 0
}

func (j *jar) Cookies(u *fasthttp.URI) []*entry {
	return j.cookies(u, time.Now())
}

func (j *jar) cookies(u *fasthttp.URI, now time.Time) (cookies []*entry) {
	if !utils.EqualFold(u.Scheme(), []byte("http")) && !utils.EqualFold(u.Scheme(), []byte("https")) {
		return
	}

	host := u.Host()
	key := jarKey(host)

	j.mu.Lock()
	defer j.mu.Unlock()

	subMap := j.entries[key]
	if subMap == nil {
		return
	}

	https := utils.EqualFold(u.Scheme(), []byte("https"))
	path := u.Path()
	if len(path) == 0 {
		path = []byte("/")
	}

	modified := false
	for id, e := range subMap {
		if e.Persistent && !e.Expires.After(now) {
			ee := subMap[id]
			delete(subMap, id)
			releaseEntry(ee)
			modified = true
			continue
		}

		if !e.shouldSend(https, host, path) {
			continue
		}

		e.LastAccess = now
		subMap[id] = e
		cookies = append(cookies, e)
		modified = true
	}

	if modified {
		if len(subMap) == 0 {
			delete(j.entries, key)
		} else {
			j.entries[key] = subMap
		}
	}

	sort.Slice(cookies, func(i, j int) bool {
		s := cookies
		if len(s[i].Path) != len(s[j].Path) {
			return len(s[i].Path) > len(s[j].Path)
		}

		if s[i].Creation != s[j].Creation {
			return s[i].Creation.Before(s[j].Creation)
		}

		return s[i].seqNum < s[j].seqNum
	})

	return
}

func (j *jar) SetCookies(u *fasthttp.URI, cookies []*fasthttp.Cookie) {
	j.setCookies(u, cookies, time.Now())
}

func (j *jar) setCookies(u *fasthttp.URI, cookies []*fasthttp.Cookie, now time.Time) {
	if len(cookies) == 0 {
		return
	}

	if !utils.EqualFold(u.Scheme(), []byte("http")) && !utils.EqualFold(u.Scheme(), []byte("https")) {
		return
	}

	host := u.Host()
	path := u.Path()
	key := jarKey(host)

	j.mu.Lock()
	defer j.mu.Unlock()

	subMap := j.entries[key]

	modified := false
	for _, cookie := range cookies {
		e, remove := newEntry(cookie, now, path)
		id := e.id()

		if remove {
			if subMap != nil {
				if _, ok := subMap[id]; ok {
					ee := subMap[id]
					delete(subMap, id)
					releaseEntry(ee)
					modified = true
				}

				continue
			}
		}

		if subMap == nil {
			subMap = make(map[string]*entry)
		}

		if old, ok := subMap[id]; ok {
			e.Creation = old.Creation
			e.seqNum = old.seqNum
		} else {
			e.Creation = now
			e.seqNum = j.nextSeqNum
			j.nextSeqNum++
		}

		e.LastAccess = now
		subMap[id] = e
		modified = true
	}

	if modified {
		if len(subMap) == 0 {
			delete(j.entries, key)
		} else {
			j.entries[key] = subMap
		}
	}
}

func jarKey(h []byte) string {
	host := utils.UnsafeString(h)
	if utils.IsIPv4(host) || utils.IsIPv6(host) {
		return host
	}

	i := strings.LastIndex(host, ".")
	if i <= 0 {
		return host
	}

	prevDot := strings.LastIndex(host[:i-1], ".")
	return host[prevDot+1:]
}

func hasDotSuffix(s, suffix []byte) bool {
	return len(s) > len(suffix) && s[len(s)-len(suffix)-1] == '.' && utils.EqualFold(s[len(s)-len(suffix):], suffix)
}

func newEntry(c *fasthttp.Cookie, now time.Time, path []byte) (*entry, bool) {
	e := acquireEntry()

	e.Key = utils.CopyBytes(c.Key())
	fmt.Println(c.Path())
	if len(c.Path()) != 0 || c.Path()[0] != '/' {
		e.Path = utils.CopyBytes(path)
	} else {
		e.Path = utils.CopyBytes(c.Path())
	}

	e.Domain = utils.CopyBytes(c.Domain())

	if c.MaxAge() < 0 {
		return e, true
	} else if c.MaxAge() > 0 {
		e.Expires = now.Add(time.Duration(c.MaxAge()) * time.Second)
		e.Persistent = true
	} else {
		if c.Expire().IsZero() {
			e.Expires = endOfTime
			e.Persistent = false
		} else {
			if !c.Expire().After(now) {
				return e, true
			}

			e.Expires = c.Expire()
			e.Persistent = true
		}
	}

	e.Value = utils.CopyBytes(c.Value())
	e.Secure = c.Secure()
	e.HttpOnly = c.HTTPOnly()

	e.SameSite = c.SameSite()

	return e, false
}

func newJar() *jar {
	return &jar{
		mu:         sync.Mutex{},
		entries:    map[string]map[string]*entry{},
		nextSeqNum: 0,
	}
}

var entryPool = &sync.Pool{
	New: func() any {
		return &entry{}
	},
}

func acquireEntry() *entry {
	e := entryPool.Get().(*entry)
	return e
}

func releaseEntry(e *entry) {
	e.reset()
	entryPool.Put(e)
}
