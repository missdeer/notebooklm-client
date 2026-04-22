package transport

import (
	"crypto/tls"
)

var ChromeCiphers = []uint16{
	tls.TLS_AES_128_GCM_SHA256,
	tls.TLS_AES_256_GCM_SHA384,
	tls.TLS_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_RSA_WITH_AES_256_CBC_SHA,
}

var ChromeSigAlgs = []tls.SignatureScheme{
	tls.ECDSAWithP256AndSHA256,
	tls.PSSWithSHA256,
	tls.PKCS1WithSHA256,
	tls.ECDSAWithP384AndSHA384,
	tls.PSSWithSHA384,
	tls.PKCS1WithSHA384,
	tls.PSSWithSHA512,
	tls.PKCS1WithSHA512,
}

func ChromeTLSConfig(serverName string) *tls.Config {
	return &tls.Config{
		ServerName:         serverName,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		CipherSuites:       ChromeCiphers,
		NextProtos:         []string{"h2", "http/1.1"},
		InsecureSkipVerify: false,
	}
}
