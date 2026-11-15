// SPDX-License-Identifier: MIT

package site

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"

	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/format"
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
	// A socket left by a server that did not stop is replaced; anything
	// else at path (a mistyped --control-socket) is the operator's.
	if fi, err := os.Lstat(path); err == nil {
		if fi.Mode()&fs.ModeSocket == 0 {
			return fmt.Errorf("%s exists and is not a socket", path)
		}
		_ = os.Remove(path)
	}
	// Only the owner may connect: the socket is created without group or
	// other permissions (listenUnix), and then made 0600.
	l, err := listenUnix(path)
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

func (c *Created) MarshalTextOutput() (string, error) {
	return c.Action + ": " + format.Display(c.Href), nil
}
func (c *Created) TableHeaders() []string { return []string{"ACTION", "HREF"} }
func (c *Created) TableRows() [][]string  { return [][]string{{c.Action, c.Href}} }

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
	// An interrupt ends the wait for the reply at once, not at the deadline.
	stop := context.AfterFunc(ctx, func() { _ = c.SetDeadline(time.Unix(1, 0)) })
	defer stop()
	if err := json.NewEncoder(c).Encode(req); err != nil {
		return callFailed(ctx, req.Op, "", err)
	}
	var reply Reply
	if err := json.NewDecoder(bufio.NewReader(c)).Decode(&reply); err != nil {
		return callFailed(ctx, req.Op, "no reply from the control socket: ", err)
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

// readOnly are the operations that change nothing on the server.
var readOnly = map[string]bool{"devices": true, "get": true}

// callFailed is the error of a request to the control socket whose
// connection failed once it was open. When ctx ended first, the error is
// that of ctx: an interruption, or a timeout, whose outcome is unknown for
// an operation that may have changed the server (it is not sent again).
// Otherwise the socket failed: control_socket_failed.
func callFailed(ctx context.Context, op, prefix string, err error) error {
	switch cerr := ctx.Err(); {
	case errors.Is(cerr, context.Canceled):
		return errs.New(errs.KindInterrupted, errs.CodeContextCanceled, "interrupted", err)
	case cerr != nil, errors.Is(err, os.ErrDeadlineExceeded):
		if readOnly[op] {
			return errs.New(errs.KindTimeout, errs.CodeRequestTimeout, "no reply from the control socket in time", err)
		}
		return errs.New(errs.KindTimeout, errs.CodeOutcomeUnknown,
			"no reply from the control socket in time; the server may have applied the request", err)
	}
	return errs.New(errs.KindTransport, errs.CodeControlSocketFailed, prefix+err.Error(), err)
}
