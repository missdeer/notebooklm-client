package transport

import "regexp"

// inBodyUnauthRe matches Google's in-body UNAUTHENTICATED envelope: when the
// session (at/bl) is stale, batchexecute returns HTTP 200 with
//
//	["wrb.fr","RPC_ID",null,null,null,[16],"generic"]
//
// where [16] is the gRPC UNAUTHENTICATED code. The matched signature is
// narrow enough (code 16 + "generic" marker) to avoid false positives on
// other RPC errors.
var inBodyUnauthRe = regexp.MustCompile(`"wrb\.fr"[^\]]*?,\s*null\s*,\s*null\s*,\s*null\s*,\s*\[\s*16\s*\]\s*,\s*"generic"`)

// IsInBodyUnauth returns true when the response body carries the
// UNAUTHENTICATED-on-200 signature. Callers should surface this as a
// SessionError so the refresh path kicks in.
func IsInBodyUnauth(body string) bool {
	return inBodyUnauthRe.MatchString(body)
}
