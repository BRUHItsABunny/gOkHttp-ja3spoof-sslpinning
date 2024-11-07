package gokhttp_ja3spoof_sslpinning

import (
	"context"
	"fmt"
	gokhttp "github.com/BRUHItsABunny/gOkHttp"
	gokhttp_ja3spoof "github.com/BRUHItsABunny/gOkHttp-ja3spoof"
	gokhttp_requests "github.com/BRUHItsABunny/gOkHttp/requests"
	gokhttp_responses "github.com/BRUHItsABunny/gOkHttp/responses"
	oohttp "github.com/ooni/oohttp"
	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewSSLPinningOption(t *testing.T) {
	pinner := NewSSLPinningOption()
	err := pinner.AddPin("tls.peet.ws", false, "sha256\\EOTuPQQdoVYMr0N3xm/wxw1AO07cihwBSQAV6P9n+oo=")
	require.NoError(t, err, "pinner.AddPin: errored unexpectedly.")

	opt := gokhttp_ja3spoof.NewJa3SpoofingOptionV2(nil, &utls.HelloChrome_120_PQ)

	hClient, err := gokhttp.NewHTTPClient(
		opt,
		pinner,
		// Public non-intercepting SOCKS5 proxy
		// gokhttp_client.NewProxyOption("http://66.29.154.105:1080"),
		// Public non-intercepting HTTP proxy
		// gokhttp_client.NewProxyOption("http://15.204.161.192:18080"),
	)
	require.NoError(t, err, "gokhttp.NewHTTPClient: errored unexpectedly.")

	// HTTP 2 stuff
	hClient.Transport.(*oohttp.StdlibTransport).Transport.HasCustomInitialSettings = true
	hClient.Transport.(*oohttp.StdlibTransport).Transport.HTTP2SettingsFrameParameters = []int64{
		65536,   // HeaderTableSize
		0,       // EnablePush
		-1,      // MaxConcurrentStreams
		6291456, // InitialWindowSize
		-1,      // MaxFrameSize
		262144,  // MaxHeaderListSize
	}

	hClient.Transport.(*oohttp.StdlibTransport).Transport.HasCustomWindowUpdate = true
	hClient.Transport.(*oohttp.StdlibTransport).Transport.WindowUpdateIncrement = 15663105
	hClient.Transport.(*oohttp.StdlibTransport).Transport.HTTP2PriorityFrameSettings = &oohttp.HTTP2PriorityFrameSettings{
		HeaderFrame: &oohttp.HTTP2Priority{
			StreamDep: 0,
			Exclusive: true,
			Weight:    255,
		},
	}

	req, err := gokhttp_requests.MakeGETRequest(context.Background(), "https://tls.peet.ws/api/all")
	require.NoError(t, err, "requests.MakeGETRequest: errored unexpectedly.")

	resp, err := hClient.Do(req)
	require.NoError(t, err, "hClient.Do: errored unexpectedly.")

	respBody, err := gokhttp_responses.ResponseText(resp)
	require.NoError(t, err, "gokhttp_responses.ResponseText: errored unexpectedly.")

	fmt.Println(respBody)
}
