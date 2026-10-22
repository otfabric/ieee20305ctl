// SPDX-License-Identifier: MIT

package site

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"

	"github.com/otfabric/ieee20305ctl/internal/errs"
)

// Request is a request on the control socket: one JSON object per line.
type Request struct {
	Op      string          `json:"op"`
	Href    string          `json:"href,omitempty"`
	Program string          `json:"program,omitempty"`
	Set     []string        `json:"set,omitempty"`
	LFDI    string          `json:"lfdi,omitempty"`
	PIN     uint32          `json:"pin,omitempty"`
	Control *ControlRequest `json:"control,omitempty"`
	Device  *DeviceSeed     `json:"device,omitempty"`
}

// Reply is the answer to a Request: one JSON object per line.
type Reply struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ReplyError     `json:"error,omitempty"`
}

// ReplyError is a failed operation.
type ReplyError struct {
	Kind    string `json:"kind"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ServeControl answers requests on the Unix socket at path until ctx ends.
func (s *Site) ServeControl(ctx context.Context, path string) error {
	_ = os.Remove(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = l.Close()
		return err
	}
	go func() {
		<-ctx.Done()
		_ = l.Close()
		_ = os.Remove(path)
	}()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go s.serveConn(ctx, c)
		}
	}()
	return nil
}

func (s *Site) serveConn(ctx context.Context, c net.Conn) {
	defer func() { _ = c.Close() }()
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	enc := json.NewEncoder(c)
	for sc.Scan() {
		var req Request
		var reply Reply
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			reply.Error = &ReplyError{Kind: string(errs.KindInvalidInput), Code: errs.CodeInvalidInput, Message: "not a JSON request: " + err.Error()}
		} else {
			octx, cancel := context.WithTimeout(ctx, 30*time.Second)
			result, err := s.do(octx, req)
			cancel()
			if err != nil {
				reply.Error = replyError(err)
			} else {
				reply.OK = true
				reply.Result, _ = json.Marshal(result)
			}
		}
		if err := enc.Encode(reply); err != nil {
			return
		}
	}
}

func replyError(err error) *ReplyError {
	kind, code := errs.KindInternal, errs.CodeInternalUnexpected
	switch {
	case errors.Is(err, ieee20305.ErrNotFound):
		kind, code = errs.KindInvalidInput, errs.CodeUnknownResource
	case errors.Is(err, ieee20305.ErrInvalid), errors.Is(err, ieee20305.ErrConfiguration):
		kind, code = errs.KindInvalidInput, errs.CodeInvalidInput
	}
	return &ReplyError{Kind: string(kind), Code: code, Message: err.Error()}
}

// Element is the result of the get operation.
type Element struct {
	Href     string `json:"href"`
	Resource string `json:"resource"`
	XML      string `json:"xml"`
}

// Created is the result of an operation that changes the server: the
// href of what it created or changed. Action names it for people.
type Created struct {
	Action string `json:"action,omitempty"`
	Href   string `json:"href"`
}

func (c *Created) MarshalTextOutput() (string, error) { return c.Action + ": " + c.Href, nil }
func (c *Created) TableHeaders() []string             { return []string{"ACTION", "HREF"} }
func (c *Created) TableRows() [][]string              { return [][]string{{c.Action, c.Href}} }

func (s *Site) do(ctx context.Context, r Request) (any, error) {
	switch r.Op {
	case "devices":
		return s.Devices(ctx)
	case "get":
		x, res, err := s.Get(ctx, r.Href)
		if err != nil {
			return nil, err
		}
		return Element{Href: r.Href, Resource: res, XML: x}, nil
	case "issue-control":
		if r.Control == nil {
			return nil, fmt.Errorf("%w: issue-control needs a control", ieee20305.ErrInvalid)
		}
		href, err := s.IssueControl(ctx, *r.Control)
		return Created{Href: href}, err
	case "cancel":
		return Created{Href: r.Href}, s.Cancel(ctx, r.Href)
	case "set-default":
		prog, err := s.Program(r.Program)
		if err != nil {
			return nil, err
		}
		return Created{Href: prog + "/dderc"}, s.SetDefault(ctx, prog, r.Set)
	case "provision":
		if r.Device == nil {
			return nil, fmt.Errorf("%w: provision needs a device", ieee20305.ErrInvalid)
		}
		href, err := s.Provision(ctx, *r.Device)
		return Created{Href: href}, err
	case "register":
		return Created{Href: r.LFDI}, s.Register(ctx, r.LFDI, r.PIN)
	}
	return nil, fmt.Errorf("%w: unknown operation %q", ieee20305.ErrInvalid, r.Op)
}

// Call sends a request to the control socket at path and decodes its
// result into v.
func Call(ctx context.Context, path string, req Request, v any) error {
	var d net.Dialer
	c, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return errs.New(errs.KindTransport, errs.CodeControlSocketFailed,
			fmt.Sprintf("the control socket %s: %v (is server start running with --control-socket?)", path, err), err)
	}
	defer func() { _ = c.Close() }()
	if dl, ok := ctx.Deadline(); ok {
		_ = c.SetDeadline(dl)
	}
	if err := json.NewEncoder(c).Encode(req); err != nil {
		return errs.New(errs.KindTransport, errs.CodeControlSocketFailed, err.Error(), err)
	}
	var reply Reply
	if err := json.NewDecoder(bufio.NewReader(c)).Decode(&reply); err != nil {
		return errs.New(errs.KindTransport, errs.CodeControlSocketFailed, "no reply from the control socket: "+err.Error(), err)
	}
	if !reply.OK {
		e := reply.Error
		if e == nil {
			e = &ReplyError{Kind: string(errs.KindInternal), Code: errs.CodeInternalUnexpected, Message: "the server failed"}
		}
		return errs.New(errs.Kind(e.Kind), e.Code, e.Message, nil)
	}
	if v != nil && len(reply.Result) > 0 {
		return json.Unmarshal(reply.Result, v)
	}
	return nil
}
