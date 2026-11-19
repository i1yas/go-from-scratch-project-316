package engine

import "net/http"

type request struct {
	method    string
	altMethod string
	httpReq   *http.Request
}

func (req *request) shouldSwitchMethod(resp *http.Response) bool {
	if req.altMethod == "" {
		return false
	}

	return req.httpReq.Method == req.method && resp != nil && resp.StatusCode == 405
}

func (req *request) switchMethod() {
	req.httpReq.Method = req.altMethod
}
