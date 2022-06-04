package fiber

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp/fasthttputil"
)

// go test -run Test_App_Listen
func Test_App_Start(t *testing.T) {
	app := New()

	utils.AssertEqual(t, false, app.Start(":99999") == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Start(":4003", StartConfig{DisableStartupMessage: true}))
}

// go test -run Test_App_Listen_Prefork
func Test_App_Listen_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New()

	utils.AssertEqual(t, nil, app.Start(":99999", StartConfig{DisableStartupMessage: true, EnablePrefork: true}))
}

// go test -run Test_App_ListenTLS
func Test_App_ListenTLS(t *testing.T) {
	app := New()

	// invalid port
	utils.AssertEqual(t, false, app.Start(":99999", StartConfig{
		CertFile:    "./.github/testdata/ssl.pem",
		CertKeyFile: "./.github/testdata/ssl.key",
	}) == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Start(":0", StartConfig{
		CertFile:    "./.github/testdata/ssl.pem",
		CertKeyFile: "./.github/testdata/ssl.key",
	}))
}

// go test -run Test_App_ListenTLS_Prefork
func Test_App_ListenTLS_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New()

	// invalid key file content
	utils.AssertEqual(t, false, app.Start(":0", StartConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/template.tmpl",
	}) == nil)

	utils.AssertEqual(t, nil, app.Start(":99999", StartConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/ssl.key",
	}))
}

// go test -run Test_App_ListenMutualTLS
func Test_App_ListenMutualTLS(t *testing.T) {
	app := New()

	// invalid port
	utils.AssertEqual(t, false, app.Start(":99999", StartConfig{
		CertFile:       "./.github/testdata/ssl.pem",
		CertKeyFile:    "./.github/testdata/ssl.key",
		CertClientFile: "./.github/testdata/ca-chain.cert.pem",
	}) == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Start(":0", StartConfig{
		CertFile:       "./.github/testdata/ssl.pem",
		CertKeyFile:    "./.github/testdata/ssl.key",
		CertClientFile: "./.github/testdata/ca-chain.cert.pem",
	}))
}

// go test -run Test_App_ListenMutualTLS_Prefork
func Test_App_ListenMutualTLS_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New()

	// invalid key file content
	utils.AssertEqual(t, false, app.Start(":0", StartConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/template.html",
		CertClientFile:        "./.github/testdata/ca-chain.cert.pem",
	}) == nil)

	utils.AssertEqual(t, nil, app.Start(":99999", StartConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/ssl.key",
		CertClientFile:        "./.github/testdata/ca-chain.cert.pem",
	}))
}

// go test -run Test_App_Listener
func Test_App_Listener(t *testing.T) {
	app := New()

	go func() {
		time.Sleep(500 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	ln := fasthttputil.NewInmemoryListener()
	utils.AssertEqual(t, nil, app.Start(ln))
}

// go test -run Test_App_Listener_Prefork
func Test_App_Listener_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New()

	ln := fasthttputil.NewInmemoryListener()
	utils.AssertEqual(t, nil, app.Start(ln, StartConfig{DisableStartupMessage: true, EnablePrefork: true}))
}

func Test_App_Listener_TLS_Listener(t *testing.T) {
	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	if err != nil {
		utils.AssertEqual(t, nil, err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	ln, err := tls.Listen(NetworkTCP4, ":0", config)
	utils.AssertEqual(t, nil, err)

	app := New()

	go func() {
		time.Sleep(time.Millisecond * 500)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Start(ln))
}
