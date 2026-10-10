package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Variants share a manifest while their bodies occupy distinct storage keys.
// Mutations happen during the origin call, after the stale entry was read.
func Test_Cache_UncacheableVaryCleanup(t *testing.T) {
	t.Parallel()

	for _, external := range []bool{false, true} {
		for _, directive := range []string{"private", "no-cache", "no-store", "VaryStar"} {
			for _, scenario := range []string{"success", "absent", "readError", "metadataDeleteError", "bodyDeleteError", "replaced", "plainNoStoreMiss"} {
				if scenario == "plainNoStoreMiss" && directive != "no-store" {
					continue
				}
				if !external && (scenario == "readError" || scenario == "metadataDeleteError" || scenario == "bodyDeleteError") {
					continue
				}
				name := "memory/" + directive + "/" + scenario
				if external {
					name = "external/" + directive + "/" + scenario
				}
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					storage := newFailingCacheStorage()
					clock := newTestClock(time.Now())
					cfg := Config{
						Expiration: time.Hour,
						MaxBytes:   10,
						clock:      clock.Now,
						KeyGenerator: func(c fiber.Ctx) string {
							return c.Path()
						},
					}
					if external {
						cfg.Storage = storage
					}
					baseKey := cacheKeyVersion + "|GET|/cached"
					manifestKey := baseKey + "|vary"
					var header fasthttp.RequestHeader
					header.Set("X-Variant", "a")
					key := varyKey(baseKey, []string{"x-variant"}, &header, true)
					app := fiber.New()
					app.Use(New(cfg))
					app.Get("/cached", func(c fiber.Ctx) error {
						c.Set(fiber.HeaderVary, "X-Variant")
						c.Set(fiber.HeaderCacheControl, "public, max-age=60")
						if c.Get("X-Action") == "cleanup" {
							if scenario == "replaced" || (!external && scenario == "absent") {
								variant := "a"
								if scenario == "absent" {
									variant = "c" // Evicts a, whose expiry precedes b's.
								}
								request := httptest.NewRequest(fiber.MethodGet, "/cached", http.NoBody)
								request.Header.Set("X-Variant", variant)
								request.Header.Set(fiber.HeaderCacheControl, "max-age=0")
								response, err := app.Test(request)
								if err != nil {
									return err
								}
								if err := response.Body.Close(); err != nil {
									return fmt.Errorf("close nested response: %w", err)
								}
								if response.StatusCode != fiber.StatusOK {
									return errors.New("nested store failed")
								}
							} else if external {
								storage.mu.Lock()
								switch scenario {
								case "absent":
									delete(storage.data, key)
									delete(storage.data, key+"_body")
								case "readError":
									storage.errs["get|"+key] = errors.New("verification failed")
								case "metadataDeleteError":
									storage.errs["del|"+key] = errors.New("metadata delete failed")
								case "bodyDeleteError":
									storage.errs["del|"+key+"_body"] = errors.New("body delete failed")
								}
								storage.mu.Unlock()
							}
							if directive == "VaryStar" {
								c.Set(fiber.HeaderVary, "*")
							} else {
								c.Set(fiber.HeaderCacheControl, directive)
							}
							return c.SendString("newer")
						}
						return c.SendString("fresh")
					})

					request := func(variant, action string) (*http.Response, string) {
						t.Helper()
						req := httptest.NewRequest(fiber.MethodGet, "/cached", http.NoBody)
						req.Header.Set("X-Variant", variant)
						if action != "" {
							req.Header.Set("X-Action", action)
							req.Header.Set(fiber.HeaderCacheControl, "max-age=0")
						}
						response, err := app.Test(req)
						require.NoError(t, err)
						body, err := io.ReadAll(response.Body)
						require.NoError(t, err)
						require.NoError(t, response.Body.Close())
						return response, string(body)
					}
					for _, variant := range []string{"a", "b"} {
						response, _ := request(variant, "")
						require.Equal(t, cacheMiss, response.Header.Get("X-Cache"))
						clock.Add(time.Second)
					}
					variant := "a"
					if scenario == "plainNoStoreMiss" {
						variant = "c"
					}
					response, body := request(variant, "cleanup")
					require.Equal(t, fiber.StatusOK, response.StatusCode)
					require.Equal(t, "newer", body)
					require.Equal(t, cacheUnreachable, response.Header.Get("X-Cache"))
					preserveManifest := scenario == "replaced" || scenario == "plainNoStoreMiss"
					if external {
						storage.mu.Lock()
						_, manifestPresent := storage.data[manifestKey]
						storage.errs = make(map[string]error)
						storage.mu.Unlock()
						require.Equal(t, preserveManifest, manifestPresent)
					}
					response, body = request("b", "")
					require.Equal(t, fiber.StatusOK, response.StatusCode)
					require.Equal(t, "fresh", body)
					wantStatus := cacheMiss
					if preserveManifest {
						wantStatus = cacheHit
					}
					require.Equal(t, wantStatus, response.Header.Get("X-Cache"))
					if scenario == "replaced" {
						response, _ = request("a", "")
						require.Equal(t, cacheHit, response.Header.Get("X-Cache"))
					}
				})
			}
		}
	}
}

func Test_CacheStorage_PartialEvictionRetainsBodyRetry(t *testing.T) {
	t.Parallel()

	for _, failBodyRead := range []bool{false, true} {
		name := "bodyPresent"
		if failBodyRead {
			name = "bodyReadError"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			storage := newFailingCacheStorage()
			probe := &accountingProbe{}
			key := cacheKeyVersion + "|GET|/a"
			app := fiber.New()
			app.Use(New(Config{
				Storage: storage, MaxBytes: 15, Expiration: time.Hour,
				accounting: probe.record,
				KeyGenerator: func(c fiber.Ctx) string {
					return c.Path()
				},
			}))
			app.Get("/:name", func(c fiber.Ctx) error {
				return c.SendString("0123456789")
			})
			request := func(path string, status int) {
				t.Helper()
				response, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
				require.NoError(t, err)
				require.Equal(t, status, response.StatusCode)
				require.NoError(t, response.Body.Close())
			}
			request("/a", fiber.StatusOK)
			storage.mu.Lock()
			storage.errs["del|"+key+"_body"] = errors.New("body deletion failed")
			if failBodyRead {
				storage.errs["get|"+key+"_body"] = errors.New("body verification failed")
			}
			storage.mu.Unlock()
			request("/b", fiber.StatusInternalServerError)
			require.Equal(t, uint(10), probe.counted())
			require.Equal(t, 1, probe.nodes())
			storage.mu.Lock()
			require.NotContains(t, storage.data, key)
			require.Contains(t, storage.data, key+"_body")
			storage.errs = make(map[string]error)
			storage.mu.Unlock()
			request("/c", fiber.StatusOK)
			storage.mu.RLock()
			require.NotContains(t, storage.data, key+"_body")
			storage.mu.RUnlock()
			require.Equal(t, uint(10), probe.counted())
			require.Equal(t, 1, probe.nodes())
		})
	}
}

func Test_manager_delIf(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"absent", "deleted", "replaced"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			manager := newManager(nil, true)
			expected := &item{body: []byte("stale")}
			replacement := &item{body: []byte("fresh")}
			want := memoryEntryAbsent
			switch scenario {
			case "deleted":
				want = memoryEntryDeleted
				manager.memory.Set("key", expected, time.Hour)
			case "replaced":
				want = memoryEntryReplaced
				manager.memory.Set("key", replacement, time.Hour)
			}
			require.Equal(t, want, manager.delIf("key", expected))
			if scenario == "replaced" {
				require.Same(t, replacement, manager.memory.Get("key"))
			} else {
				_, err := manager.get(context.Background(), "key")
				require.ErrorIs(t, err, errCacheMiss)
			}
		})
	}
}
