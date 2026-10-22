// SPDX-License-Identifier: MIT

package site

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
	"github.com/otfabric/go-ieee20305/csipaus"
	"github.com/otfabric/go-ieee20305/csipaus/csipserver"
	"github.com/otfabric/go-ieee20305/schedule"
	"github.com/otfabric/go-ieee20305/security"
	"github.com/otfabric/go-ieee20305/server"
	"github.com/otfabric/go-ieee20305/tlsccm"

	"github.com/otfabric/ieee20305ctl/internal/conn"
	"github.com/otfabric/ieee20305ctl/internal/ieee"
	"github.com/otfabric/ieee20305ctl/internal/types"
)

// Options configure a Site.
type Options struct {
	// Profile is ieee20305 or csipaus.
	Profile string
	// Edition is the edition of devices the seed does not give one.
	Edition ieee20305.Edition
	// InBand lets unknown devices post their EndDevice.
	InBand bool
	// ServerCert and DeviceCAs are the server's certificate, which its
	// notifier presents too, and the CAs of its devices.
	ServerCert tls.Certificate
	DeviceCAs  *x509.CertPool
	// CCM8 says the server is behind the CCM_8 listener of tlsccm.
	CCM8 bool
	// NotifyLoopback lets devices subscribe with notification URIs on
	// loopback and link-local addresses, which the server refuses by
	// default.
	NotifyLoopback bool
	// LogRequests reports every request as an event.
	LogRequests bool
	// Events receives what happens; it must not block.
	Events func(*types.Event)
}

// Site is a running server with what was seeded.
type Site struct {
	Server *server.Server
	admin  *server.Admin
	opts   Options
	codec  ieee20305.Codec

	mu        sync.Mutex
	responses string
	programs  []program
	tariffs   []string
	seedPoll  uint32
}

type program struct {
	description, href string
}

// New starts the server of opts, seeded with seed.
func New(ctx context.Context, opts Options, seed *Seed) (*Site, error) {
	if seed == nil {
		seed = &Seed{}
	}
	s := &Site{opts: opts, codec: opts.Edition.Codec(), seedPoll: seed.PollRate}
	cfg := server.Config{
		DefaultEdition:     opts.Edition,
		InBandRegistration: opts.InBand,
		PollRate:           seed.PollRate,
		OnResponse:         s.onResponse,
		Metrics:            metrics{s},
		Logger:             logger{s},
		NotificationClient: server.NewNotificationClient(opts.ServerCert, opts.DeviceCAs),
	}
	if opts.NotifyLoopback {
		cfg.NotificationURIAllowed = func(*url.URL) bool { return true }
	}
	if opts.CCM8 {
		cfg.PeerCertificates = tlsccm.PeerCertificates
	}
	if opts.Profile == "csipaus" {
		cfg = csipserver.Configure(cfg)
		s.codec = csipaus.Codec{}
	}
	srv, err := server.New(cfg)
	if err != nil {
		return nil, err
	}
	s.Server, s.admin = srv, srv.Admin()
	if err := s.apply(ctx, seed); err != nil {
		_ = srv.Close()
		return nil, err
	}
	return s, nil
}

// Close stops the server's background work.
func (s *Site) Close() error { return s.Server.Close() }

func (s *Site) event(e *types.Event) {
	if s.opts.Events != nil {
		e.Time = time.Now().UTC()
		s.opts.Events(e)
	}
}

func (s *Site) aus() bool { return s.opts.Profile == "csipaus" }

func (s *Site) apply(ctx context.Context, seed *Seed) error {
	a := s.admin
	for _, r := range []struct{ resource, href string }{
		{"DERProgramList", "/derp"}, {"ResponseSetList", "/rsps"}, {"MirrorUsagePointList", "/mup"}, {"TariffProfileList", "/tp"},
	} {
		if err := a.Root(ctx, r.resource, r.href, a.Document(r.resource)); err != nil {
			return err
		}
	}
	rs, err := a.Create(ctx, "/rsps", &ieee20305.ResponseSet{}, "")
	if err != nil {
		return err
	}
	s.responses = rs + "/rsp"
	start := a.Now()
	for i, p := range seed.Programs {
		if err := s.addProgram(ctx, p, start); err != nil {
			return fmt.Errorf("programs[%d]: %w", i, err)
		}
	}
	for i, t := range seed.Tariffs {
		if err := s.addTariff(ctx, t, start); err != nil {
			return fmt.Errorf("tariffs[%d]: %w", i, err)
		}
	}
	for i, d := range seed.Devices {
		if _, err := s.Provision(ctx, d); err != nil {
			return fmt.Errorf("devices[%d]: %w", i, err)
		}
	}
	return nil
}

func (s *Site) addProgram(ctx context.Context, p ProgramSeed, start time.Time) error {
	id, err := mrid(p.MRID, "program "+p.Description)
	if err != nil {
		return err
	}
	prog := &ieee20305.DERProgram{Primacy: ieee20305.PrimacyType(p.Primacy)}
	prog.MRID = id
	desc := ieee20305.String32(p.Description)
	prog.Description = &desc
	href, err := s.admin.Create(ctx, "/derp", prog, "")
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.programs = append(s.programs, program{p.Description, href})
	s.mu.Unlock()
	if len(p.Default) > 0 {
		if err := s.SetDefault(ctx, href, p.Default.Pairs()); err != nil {
			return err
		}
	}
	for j, c := range p.Controls {
		if _, err := s.IssueControl(ctx, ControlRequest{Program: href, Description: c.Description, MRID: c.MRID,
			Set: c.Set.Pairs(), StartAt: start.Add(time.Duration(c.Start)), Duration: time.Duration(c.Duration),
			NoResponses: c.NoResponses}); err != nil {
			return fmt.Errorf("controls[%d]: %w", j, err)
		}
	}
	return nil
}

// ControlRequest is a DER control to issue.
type ControlRequest struct {
	// Program is the program's href or description.
	Program     string        `json:"program"`
	Description string        `json:"description,omitempty"`
	MRID        string        `json:"mrid,omitempty"`
	Set         []string      `json:"set"`
	StartAt     time.Time     `json:"start_at"`
	Duration    time.Duration `json:"duration"`
	NoResponses bool          `json:"no_responses,omitempty"`
}

// Program returns the href of a program by its href or description.
func (s *Site) Program(name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.programs {
		if p.href == name || p.description == name {
			return p.href, nil
		}
	}
	if len(s.programs) == 1 && name == "" {
		return s.programs[0].href, nil
	}
	return "", fmt.Errorf("%w: no DER program %q", ieee20305.ErrNotFound, name)
}

// IssueControl creates a DER control in a program, and returns its href.
func (s *Site) IssueControl(ctx context.Context, r ControlRequest) (string, error) {
	prog, err := s.Program(r.Program)
	if err != nil {
		return "", err
	}
	if r.Duration <= 0 {
		return "", fmt.Errorf("%w: a control needs a positive duration", ieee20305.ErrInvalid)
	}
	base, err := s.parse(r.Set)
	if err != nil {
		return "", err
	}
	id, err := mrid(r.MRID, fmt.Sprintf("control %s %s %d", prog, r.Description, time.Now().UnixNano()))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ieee20305.ErrInvalid, err)
	}
	if r.StartAt.IsZero() {
		r.StartAt = s.admin.Now()
	}
	r.StartAt = r.StartAt.UTC().Truncate(time.Second)
	c := &csipaus.DERControl{DERControlBase: base}
	c.MRID = id
	if r.Description != "" {
		d := ieee20305.String32(r.Description)
		c.Description = &d
	}
	c.CreationTime = ieee20305.TimeType(s.admin.Now().Unix())
	c.Interval = ieee20305.DateTimeInterval{Start: ieee20305.TimeType(r.StartAt.Unix()), Duration: ieee20305.UInt32(r.Duration / time.Second)}
	if !r.NoResponses {
		replyTo, flags := s.responses, ieee20305.HexBinary8{schedule.RespondReceived | schedule.RespondSpecific}
		c.ReplyTo, c.ResponseRequired = &replyTo, &flags
	}
	var doc any = c
	if !s.aus() {
		doc = &ieee20305.DERControl{RandomizableEvent: c.RandomizableEvent, DERControlBase: c.DERControlBase.DERControlBase}
	}
	href, err := s.admin.Create(ctx, prog+"/derc", doc, "")
	if err != nil {
		return "", err
	}
	end := r.StartAt.Add(r.Duration)
	s.event(&types.Event{Kind: "control_issued", Href: href, MRID: strings.ToUpper(hex.EncodeToString(id)), Resource: prog,
		Settings: ieee.Settings(&base), Start: &r.StartAt, End: &end})
	return href, nil
}

// parse parses settings, refusing those of CSIP-AUS on a server that is not
// of the profile, which could not serve them.
func (s *Site) parse(set []string) (csipaus.DERControlBase, error) {
	base, err := ieee.ParseSettings(set)
	if err != nil {
		return base, fmt.Errorf("%w: %v", ieee20305.ErrInvalid, err)
	}
	if !s.aus() {
		if names := ieee.CSIPAUSOnly(&base); len(names) > 0 {
			return base, fmt.Errorf("%w: %s are CSIP-AUS settings: start the server with --profile csipaus", ieee20305.ErrInvalid, strings.Join(names, ", "))
		}
	}
	return base, nil
}

// SetDefault puts the default control of a program.
func (s *Site) SetDefault(ctx context.Context, programName string, set []string) error {
	prog, err := s.Program(programName)
	if err != nil {
		return err
	}
	base, err := s.parse(set)
	if err != nil {
		return err
	}
	d := &csipaus.DefaultDERControl{DERControlBase: base}
	d.MRID, _ = mrid("", "default "+prog)
	now := ieee20305.TimeType(s.admin.Now().Unix())
	d.UpdatedTime = &now
	var doc any = d
	if !s.aus() {
		doc = &ieee20305.DefaultDERControl{RespondableSubscribableIdentifiedObject: d.RespondableSubscribableIdentifiedObject,
			DERControlBase: base.DERControlBase, UpdatedTime: &now}
	}
	if err := s.admin.Put(ctx, &server.Node{Href: prog + "/dderc", Resource: "DefaultDERControl", Value: doc}); err != nil {
		return err
	}
	s.event(&types.Event{Kind: "default_set", Href: prog + "/dderc", Settings: ieee.Settings(&base)})
	return nil
}

// Cancel cancels an event.
func (s *Site) Cancel(ctx context.Context, href string) error {
	if err := s.admin.CancelEvent(ctx, href, false); err != nil {
		return err
	}
	s.event(&types.Event{Kind: "control_cancelled", Href: href})
	return nil
}

func (s *Site) addTariff(ctx context.Context, t TariffSeed, start time.Time) error {
	a := s.admin
	id, err := mrid(t.MRID, "tariff "+t.Description)
	if err != nil {
		return err
	}
	tp := &ieee20305.TariffProfile{Primacy: ieee20305.PrimacyType(t.Primacy)}
	tp.MRID = id
	desc := ieee20305.String32(t.Description)
	tp.Description = &desc
	cur, mult := ieee20305.CurrencyCode(t.Currency), ieee20305.PowerOfTenMultiplierType(t.PriceMultiplier)
	tp.Currency, tp.PricePowerOfTenMultiplier = &cur, &mult
	if t.RateCode != "" {
		rc := ieee20305.String20(t.RateCode)
		tp.RateCode = &rc
	}
	var tdoc any = tp
	if s.aus() {
		tdoc = &csipaus.TariffProfile{TariffProfile: *tp}
	}
	tph, err := a.Create(ctx, "/tp", tdoc, "")
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.tariffs = append(s.tariffs, tph)
	s.mu.Unlock()
	for _, rcs := range t.RateComponents {
		flags, err := hex.DecodeString(rcs.RoleFlags)
		if err != nil {
			return fmt.Errorf("role_flags %q: %w", rcs.RoleFlags, err)
		}
		if len(flags) == 0 {
			flags = []byte{0x00, 0x01}
		}
		rc := &ieee20305.RateComponent{RoleFlags: flags}
		if rc.MRID, err = mrid(rcs.MRID, "rate component "+t.Description+" "+rcs.Description); err != nil {
			return err
		}
		d := ieee20305.String32(rcs.Description)
		rc.Description = &d
		rch, err := a.Create(ctx, tph+"/rc", rc, "")
		if err != nil {
			return err
		}
		dir := ieee20305.FlowDirectionType(rcs.FlowDirection)
		if err := a.Put(ctx, &server.Node{Href: rch + "/rt", Resource: "ReadingType", Value: &ieee20305.ReadingType{FlowDirection: &dir}}); err != nil {
			return err
		}
		n, err := a.Get(ctx, rch)
		if err != nil {
			return err
		}
		n.Value.(*ieee20305.RateComponent).ReadingTypeLink.Href = rch + "/rt"
		if err := a.Put(ctx, n); err != nil {
			return err
		}
		for _, iv := range rcs.Intervals {
			tti := &ieee20305.TimeTariffInterval{TouTier: ieee20305.TOUType(max(iv.TOUTier, 1))}
			if tti.MRID, err = mrid(iv.MRID, "interval "+rch+" "+iv.Description); err != nil {
				return err
			}
			d := ieee20305.String32(iv.Description)
			tti.Description = &d
			tti.CreationTime = ieee20305.TimeType(start.Unix())
			at := start.Add(time.Duration(iv.Start))
			tti.Interval = ieee20305.DateTimeInterval{Start: ieee20305.TimeType(at.Unix()), Duration: ieee20305.UInt32(time.Duration(iv.Duration) / time.Second)}
			var idoc any = tti
			if s.aus() {
				idoc = &csipaus.TimeTariffInterval{TimeTariffInterval: *tti, RateComponentLink: ieee20305.RateComponentLink{Link: ieee20305.Link{Href: rch}}}
			}
			ttih, err := a.Create(ctx, rch+"/tti", idoc, "")
			if err != nil {
				return err
			}
			price := ieee20305.Int32(iv.Price)
			if _, err := a.Create(ctx, ttih+"/cti", &ieee20305.ConsumptionTariffInterval{ConsumptionBlock: 1, Price: &price}, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// Provision tells the server of a device: it registers it (unless
// Unregistered) and, for a device, assigns it the seeded programs and
// tariffs. It returns the href of its EndDevice.
func (s *Site) Provision(ctx context.Context, d DeviceSeed) (string, error) {
	var lfdi security.LFDI
	switch {
	case d.Cert != "":
		c, err := conn.LoadCertificate(d.Cert)
		if err != nil {
			return "", err
		}
		lfdi = security.CertificateLFDI(c)
	default:
		var err error
		if lfdi, err = security.ParseLFDI(d.LFDI); err != nil {
			return "", err
		}
	}
	spec := server.DeviceSpec{LFDI: lfdi, Edition: s.opts.Edition, Register: !d.Unregistered, PIN: d.PIN}
	if d.Edition != "" {
		e, err := ieee20305.ParseEdition(d.Edition)
		if err != nil {
			return "", err
		}
		spec.Edition = e
	}
	if s.aus() {
		spec.Edition = ieee20305.Edition2018
	}
	switch d.Role {
	case "aggregator":
		spec.Role = server.RoleAggregator
	case "operator":
		spec.Role = server.RoleOperator
	}
	if d.ManagedBy != "" {
		m, err := security.ParseLFDI(d.ManagedBy)
		if err != nil {
			return "", fmt.Errorf("managed_by: %w", err)
		}
		spec.ManagedBy = m
	}
	if d.DeviceCategory != "" {
		b, err := hex.DecodeString(d.DeviceCategory)
		if err != nil {
			return "", fmt.Errorf("device_category %q: %w", d.DeviceCategory, err)
		}
		dc := ieee20305.DeviceCategoryType(b)
		spec.EndDevice = &ieee20305.EndDevice{}
		spec.EndDevice.DeviceCategory = &dc
	}
	ed, err := s.admin.ProvisionDevice(ctx, spec)
	if err != nil {
		return "", err
	}
	if spec.Role == server.RoleDevice {
		if err := s.assign(ctx, lfdi, ed); err != nil {
			return "", err
		}
	}
	s.event(&types.Event{Kind: "device_provisioned", Href: ed, LFDI: lfdi.String(),
		Message: fmt.Sprintf("SFDI %s, %s, %s", lfdi.SFDI(), roleName(spec.Role), spec.Edition)})
	return ed, nil
}

func roleName(r server.Role) string {
	switch r {
	case server.RoleAggregator:
		return "aggregator"
	case server.RoleOperator:
		return "operator"
	default:
		return "device"
	}
}

// assign gives a device a DER and a FunctionSetAssignments with the
// seeded programs and tariffs.
func (s *Site) assign(ctx context.Context, lfdi security.LFDI, ed string) error {
	a := s.admin
	if _, err := a.Create(ctx, ed+"/der", &ieee20305.DER{}, lfdi.String()); err != nil {
		return err
	}
	fsaDoc := &ieee20305.FunctionSetAssignments{}
	fsaDoc.MRID, _ = mrid("", "fsa "+lfdi.String())
	desc := ieee20305.String32("ieee20305ctl")
	fsaDoc.Description = &desc
	fsa, err := a.AssignFSA(ctx, lfdi, fsaDoc)
	if err != nil {
		return err
	}
	s.mu.Lock()
	progs, tariffs := append([]program(nil), s.programs...), append([]string(nil), s.tariffs...)
	s.mu.Unlock()
	if err := a.NewList(ctx, "DERProgramList", fsa+"/derp", fsa, "DERProgramListLink", lfdi.String()); err != nil {
		return err
	}
	for _, p := range progs {
		if err := a.AddMember(ctx, fsa+"/derp", p.href); err != nil {
			return err
		}
	}
	if err := a.NewList(ctx, "TariffProfileList", fsa+"/tp", fsa, "TariffProfileListLink", lfdi.String()); err != nil {
		return err
	}
	for _, t := range tariffs {
		if err := a.AddMember(ctx, fsa+"/tp", t); err != nil {
			return err
		}
	}
	return nil
}

// Register registers a device the server knows, with a PIN.
func (s *Site) Register(ctx context.Context, lfdi string, pin uint32) error {
	l, err := security.ParseLFDI(lfdi)
	if err != nil {
		return err
	}
	return s.admin.Register(ctx, l, pin)
}

// DeviceInfo is a device of the server.
type DeviceInfo struct {
	EndDevice  string `json:"end_device"`
	LFDI       string `json:"lfdi"`
	SFDI       string `json:"sfdi"`
	Registered bool   `json:"registered"`
	PIN        uint32 `json:"pin,omitempty"`
	Changed    string `json:"changed,omitempty"`
}

// Devices lists the EndDevices of the server.
func (s *Site) Devices(ctx context.Context) ([]DeviceInfo, error) {
	nodes, err := s.admin.Members(ctx, server.EndDeviceListHref)
	if err != nil {
		return nil, err
	}
	out := []DeviceInfo{}
	for _, n := range nodes {
		var ed *ieee20305.EndDevice
		switch v := n.Value.(type) {
		case *ieee20305.EndDevice:
			ed = v
		case *csipaus.EndDevice:
			ed = &v.EndDevice
		default:
			continue
		}
		info := DeviceInfo{EndDevice: n.Href, SFDI: fmt.Sprint(ed.SFDI)}
		if ed.LFDI != nil {
			info.LFDI = strings.ToUpper(hex.EncodeToString(*ed.LFDI))
		}
		if ed.ChangedTime != 0 {
			info.Changed = time.Unix(int64(ed.ChangedTime), 0).UTC().Format(time.RFC3339)
		}
		if rg, err := s.admin.Get(ctx, n.Href+"/rg"); err == nil {
			if r, ok := rg.Value.(*ieee20305.Registration); ok {
				info.Registered, info.PIN = true, uint32(r.PIN)
			}
		}
		out = append(out, info)
	}
	return out, nil
}

// Get returns the document at href, encoded in the server's profile.
func (s *Site) Get(ctx context.Context, href string) (string, string, error) {
	n, err := s.admin.Get(ctx, href)
	if err != nil {
		return "", "", err
	}
	if n.Value == nil {
		return "", n.Resource, fmt.Errorf("%w: %s has no value yet", ieee20305.ErrNotFound, href)
	}
	data, err := s.codec.Marshal(n.Value)
	if err != nil {
		return "", n.Resource, err
	}
	return string(data), n.Resource, nil
}

func (s *Site) onResponse(_ context.Context, href string, r any) {
	var resp *ieee20305.Response
	switch v := r.(type) {
	case *ieee20305.DERControlResponse:
		resp = &v.Response
	case *ieee20305.Response:
		resp = v
	case *ieee20305.DrResponse:
		resp = &v.Response
	case *ieee20305.PriceResponse:
		resp = &v.Response
	case *ieee20305.TextResponse:
		resp = &v.Response
	}
	e := &types.Event{Kind: "response", Href: href}
	if resp != nil {
		e.MRID = strings.ToUpper(hex.EncodeToString(resp.Subject))
		e.LFDI = strings.ToUpper(hex.EncodeToString(resp.EndDeviceLFDI))
		if resp.Status != nil {
			e.Code = int(*resp.Status)
		}
	}
	s.event(e)
}

// metrics reports the server's requests, notifications and event
// transitions as events.
type metrics struct{ s *Site }

func (m metrics) OnRequest(string) {}
func (m metrics) OnResponse(method, resource string, status int, d time.Duration) {
	if m.s.opts.LogRequests || status >= 500 {
		m.s.event(&types.Event{Kind: "request", Status: method, Resource: resource, Code: status, Message: d.Round(time.Microsecond).String()})
	}
}
func (m metrics) OnNotification(resource string, d time.Duration, err error) {
	e := &types.Event{Kind: "notification", Resource: resource, Message: d.Round(time.Millisecond).String()}
	if err != nil {
		e.Kind, e.Message = "notification_failed", err.Error()
	}
	m.s.event(e)
}
func (m metrics) OnSubscriptionCancelled(resource string, status uint8) {
	m.s.event(&types.Event{Kind: "subscription_cancelled", Resource: resource, Code: int(status)})
}
func (m metrics) OnEventTransition(element string, _, to schedule.Status) {
	m.s.event(&types.Event{Kind: "event_status", Resource: element, Status: to.String()})
}

// logger reports what the server logs at Info and above as events.
type logger struct{ s *Site }

func (l logger) Debugf(string, ...any) {}
func (l logger) Infof(f string, a ...any) {
	l.s.event(&types.Event{Kind: "log", Message: fmt.Sprintf(f, a...)})
}
func (l logger) Warnf(f string, a ...any) {
	l.s.event(&types.Event{Kind: "warning", Message: fmt.Sprintf(f, a...)})
}
func (l logger) Errorf(f string, a ...any) {
	l.s.event(&types.Event{Kind: "error", Message: fmt.Sprintf(f, a...)})
}
