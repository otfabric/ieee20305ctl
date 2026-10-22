// SPDX-License-Identifier: MIT

// Package types holds the results of the commands of ieee20305ctl: what
// --format json writes, field by field (docs/json-output.md), and how text
// and tables render them.
package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/otfabric/ieee20305ctl/internal/ieee"
)

// Link is a link of a resource.
type Link struct {
	Name string  `json:"name"`
	Href string  `json:"href"`
	All  *uint32 `json:"all,omitempty"`
}

// Capability is the result of client dcap: the server's DeviceCapability.
type Capability struct {
	URL      string `json:"url"`
	PollRate uint32 `json:"poll_rate_s"`
	Links    []Link `json:"links"`
}

func (c *Capability) MarshalTextOutput() (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "DeviceCapability %s (poll every %d s)\n", c.URL, c.PollRate)
	for _, l := range c.Links {
		all := ""
		if l.All != nil {
			all = fmt.Sprintf(" (%d)", *l.All)
		}
		fmt.Fprintf(&b, "  %-34s %s%s\n", l.Name, l.Href, all)
	}
	return b.String(), nil
}

func (c *Capability) TableHeaders() []string { return []string{"LINK", "HREF", "ALL"} }
func (c *Capability) TableRows() [][]string {
	rows := [][]string{}
	for _, l := range c.Links {
		all := ""
		if l.All != nil {
			all = strconv.FormatUint(uint64(*l.All), 10)
		}
		rows = append(rows, []string{l.Name, l.Href, all})
	}
	return rows
}

// TimeResult is the result of client time.
type TimeResult struct {
	ServerTime time.Time `json:"server_time"`
	LocalTime  time.Time `json:"local_time"`
	OffsetMS   int64     `json:"offset_ms"`
	TZOffsetS  int32     `json:"tz_offset_s"`
	DSTOffsetS int32     `json:"dst_offset_s"`
	Quality    uint8     `json:"quality"`
}

func (r *TimeResult) MarshalTextOutput() (string, error) {
	return fmt.Sprintf("server time  %s\nlocal time   %s\noffset       %s (the server is %s)\ntime zone    %+d s, DST %+d s\nquality      %d\n",
		r.ServerTime.Format(time.RFC3339), r.LocalTime.Format(time.RFC3339), time.Duration(r.OffsetMS)*time.Millisecond,
		aheadBehind(r.OffsetMS), r.TZOffsetS, r.DSTOffsetS, r.Quality), nil
}

func aheadBehind(ms int64) string {
	switch {
	case ms > 0:
		return "ahead"
	case ms < 0:
		return "behind"
	}
	return "in step"
}

func (r *TimeResult) TableHeaders() []string {
	return []string{"SERVER_TIME", "OFFSET_MS", "TZ_OFFSET_S", "DST_OFFSET_S", "QUALITY"}
}
func (r *TimeResult) TableRows() [][]string {
	return [][]string{{r.ServerTime.Format(time.RFC3339), strconv.FormatInt(r.OffsetMS, 10),
		strconv.Itoa(int(r.TZOffsetS)), strconv.Itoa(int(r.DSTOffsetS)), strconv.Itoa(int(r.Quality))}}
}

// Registration is the result of client register.
type Registration struct {
	EndDevice  string     `json:"end_device"`
	Created    bool       `json:"created"`
	LFDI       string     `json:"lfdi"`
	SFDI       string     `json:"sfdi"`
	Registered bool       `json:"registered"`
	PIN        *uint32    `json:"pin,omitempty"`
	PINChecked bool       `json:"pin_checked"`
	PINMatches *bool      `json:"pin_matches,omitempty"`
	Since      *time.Time `json:"registered_at,omitempty"`
}

func (r *Registration) MarshalTextOutput() (string, error) {
	var b strings.Builder
	verb := "found"
	if r.Created {
		verb = "created"
	}
	fmt.Fprintf(&b, "EndDevice %s (%s)\n  LFDI %s\n  SFDI %s\n", r.EndDevice, verb, r.LFDI, r.SFDI)
	switch {
	case !r.Registered:
		b.WriteString("  not registered by the server yet\n")
	case r.PINChecked && r.PINMatches != nil && *r.PINMatches:
		fmt.Fprintf(&b, "  registered, PIN %d matches\n", *r.PIN)
	case r.PINChecked:
		fmt.Fprintf(&b, "  registered, PIN %d does NOT match\n", *r.PIN)
	default:
		fmt.Fprintf(&b, "  registered with PIN %d\n", *r.PIN)
	}
	return b.String(), nil
}

func (r *Registration) TableHeaders() []string {
	return []string{"END_DEVICE", "CREATED", "LFDI", "SFDI", "REGISTERED", "PIN", "PIN_MATCHES"}
}
func (r *Registration) TableRows() [][]string {
	pin, matches := "", ""
	if r.PIN != nil {
		pin = strconv.FormatUint(uint64(*r.PIN), 10)
	}
	if r.PINMatches != nil {
		matches = strconv.FormatBool(*r.PINMatches)
	}
	return [][]string{{r.EndDevice, strconv.FormatBool(r.Created), r.LFDI, r.SFDI, strconv.FormatBool(r.Registered), pin, matches}}
}

// Document is the result of client get: a resource as the server sent it.
type Document struct {
	Href    string `json:"href"`
	Element string `json:"element"`
	XML     string `json:"xml"`
}

func (d *Document) MarshalTextOutput() (string, error) { return d.XML, nil }

// WalkStep is a record of client walk.
type WalkStep struct {
	Depth   int    `json:"depth"`
	Href    string `json:"href"`
	Element string `json:"element"`
	Link    string `json:"link,omitempty"`
	Items   *int   `json:"items,omitempty"`
}

func (s *WalkStep) MarshalTextOutput() (string, error) {
	items := ""
	if s.Items != nil {
		items = fmt.Sprintf(" [%d]", *s.Items)
	}
	return fmt.Sprintf("%s%s %s%s", strings.Repeat("  ", s.Depth), s.Href, s.Element, items), nil
}

func (s *WalkStep) TableHeaders() []string {
	return []string{"DEPTH", "HREF", "ELEMENT", "LINK", "ITEMS"}
}
func (s *WalkStep) TableRows() [][]string {
	items := ""
	if s.Items != nil {
		items = strconv.Itoa(*s.Items)
	}
	return [][]string{{strconv.Itoa(s.Depth), s.Href, s.Element, s.Link, items}}
}

// Control is a DER control as a server serves it.
type Control struct {
	Program          string         `json:"program"`
	Href             string         `json:"href"`
	MRID             string         `json:"mrid"`
	Description      string         `json:"description,omitempty"`
	Start            time.Time      `json:"start"`
	DurationS        uint32         `json:"duration_s"`
	Status           string         `json:"status"`
	Settings         []ieee.Setting `json:"settings"`
	ReplyTo          string         `json:"reply_to,omitempty"`
	ResponseRequired string         `json:"response_required,omitempty"`
}

// Program is a DER program as a server serves it to the device.
type Program struct {
	Href        string         `json:"href"`
	MRID        string         `json:"mrid"`
	Description string         `json:"description,omitempty"`
	Primacy     uint8          `json:"primacy"`
	Default     []ieee.Setting `json:"default_control"`
	Controls    int            `json:"controls"`
	Active      int            `json:"active"`
}

// Programs is the result of client programs.
type Programs struct {
	Programs []Program `json:"programs"`
}

func (p *Programs) MarshalTextOutput() (string, error) {
	if len(p.Programs) == 0 {
		return "no DER programs", nil
	}
	var b strings.Builder
	for _, pr := range p.Programs {
		fmt.Fprintf(&b, "%s  primacy %d  %s\n", pr.Href, pr.Primacy, quote(pr.Description))
		fmt.Fprintf(&b, "  default   %s\n", settingsText(pr.Default))
		fmt.Fprintf(&b, "  controls  %d (%d active)\n", pr.Controls, pr.Active)
	}
	return b.String(), nil
}

func (p *Programs) TableHeaders() []string {
	return []string{"HREF", "PRIMACY", "DESCRIPTION", "DEFAULT", "CONTROLS", "ACTIVE"}
}
func (p *Programs) TableRows() [][]string {
	rows := [][]string{}
	for _, pr := range p.Programs {
		rows = append(rows, []string{pr.Href, strconv.Itoa(int(pr.Primacy)), pr.Description, settingsText(pr.Default),
			strconv.Itoa(pr.Controls), strconv.Itoa(pr.Active)})
	}
	return rows
}

// Controls is the result of client controls.
type Controls struct {
	Controls []Control `json:"controls"`
}

func (c *Controls) MarshalTextOutput() (string, error) {
	if len(c.Controls) == 0 {
		return "no DER controls", nil
	}
	var b strings.Builder
	for _, ct := range c.Controls {
		fmt.Fprintf(&b, "%s  %s  %s\n", ct.Href, ct.Status, quote(ct.Description))
		fmt.Fprintf(&b, "  %s for %s (program %s)\n", ct.Start.Format(time.RFC3339), time.Duration(ct.DurationS)*time.Second, ct.Program)
		fmt.Fprintf(&b, "  %s\n", settingsText(ct.Settings))
		if ct.ReplyTo != "" {
			fmt.Fprintf(&b, "  responses %s to %s\n", ct.ResponseRequired, ct.ReplyTo)
		}
	}
	return b.String(), nil
}

func (c *Controls) TableHeaders() []string {
	return []string{"HREF", "STATUS", "START", "DURATION_S", "SETTINGS", "DESCRIPTION"}
}
func (c *Controls) TableRows() [][]string {
	rows := [][]string{}
	for _, ct := range c.Controls {
		rows = append(rows, []string{ct.Href, ct.Status, ct.Start.Format(time.RFC3339), strconv.FormatUint(uint64(ct.DurationS), 10),
			settingsText(ct.Settings), ct.Description})
	}
	return rows
}

func settingsText(s []ieee.Setting) string {
	if len(s) == 0 {
		return "(none)"
	}
	parts := make([]string, len(s))
	for i, x := range s {
		parts[i] = x.Name + "=" + x.Text
	}
	return strings.Join(parts, " ")
}

func quote(s string) string {
	if s == "" {
		return ""
	}
	return strconv.Quote(s)
}

// Posted is the result of a command that creates a resource: client
// respond, log-event, report, flow-reservation, post.
type Posted struct {
	Action   string `json:"action"`
	Target   string `json:"target"`
	Location string `json:"location,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

func (p *Posted) MarshalTextOutput() (string, error) {
	s := fmt.Sprintf("%s: posted to %s", p.Action, p.Target)
	if p.Location != "" {
		s += ", created " + p.Location
	}
	if p.Detail != "" {
		s += "\n  " + p.Detail
	}
	return s, nil
}

func (p *Posted) TableHeaders() []string { return []string{"ACTION", "TARGET", "LOCATION", "DETAIL"} }
func (p *Posted) TableRows() [][]string {
	return [][]string{{p.Action, p.Target, p.Location, p.Detail}}
}

// Written is the result of client put, client delete and client der put.
type Written struct {
	Action string   `json:"action"`
	Hrefs  []string `json:"hrefs"`
}

func (w *Written) MarshalTextOutput() (string, error) {
	return fmt.Sprintf("%s: %s", w.Action, strings.Join(w.Hrefs, ", ")), nil
}

func (w *Written) TableHeaders() []string { return []string{"ACTION", "HREF"} }
func (w *Written) TableRows() [][]string {
	rows := [][]string{}
	for _, h := range w.Hrefs {
		rows = append(rows, []string{w.Action, h})
	}
	return rows
}

// DERResource is one of the resources of a DER.
type DERResource struct {
	Name string `json:"name"`
	Href string `json:"href"`
	// Set reports whether the device has put it.
	Set bool   `json:"set"`
	XML string `json:"xml,omitempty"`
}

// DERs is the result of client der show.
type DERs struct {
	DERs []DER `json:"ders"`
}

// DER is a DER of the device, with its resources.
type DER struct {
	Href      string        `json:"href"`
	Resources []DERResource `json:"resources"`
}

func (d *DERs) MarshalTextOutput() (string, error) {
	if len(d.DERs) == 0 {
		return "no DER", nil
	}
	var b strings.Builder
	for _, der := range d.DERs {
		fmt.Fprintf(&b, "DER %s\n", der.Href)
		for _, r := range der.Resources {
			state := "not set"
			if r.Set {
				state = "set"
			}
			fmt.Fprintf(&b, "  %-16s %-28s %s\n", r.Name, r.Href, state)
			if r.XML != "" {
				for _, line := range strings.Split(strings.TrimSpace(r.XML), "\n") {
					fmt.Fprintf(&b, "    %s\n", line)
				}
			}
		}
	}
	return b.String(), nil
}

func (d *DERs) TableHeaders() []string { return []string{"DER", "RESOURCE", "HREF", "SET"} }
func (d *DERs) TableRows() [][]string {
	rows := [][]string{}
	for _, der := range d.DERs {
		for _, r := range der.Resources {
			rows = append(rows, []string{der.Href, r.Name, r.Href, strconv.FormatBool(r.Set)})
		}
	}
	return rows
}

// Interval is a time tariff interval with its first price.
type Interval struct {
	Href        string    `json:"href"`
	Description string    `json:"description,omitempty"`
	Start       time.Time `json:"start"`
	DurationS   uint32    `json:"duration_s"`
	Status      string    `json:"status"`
	TOUTier     uint8     `json:"tou_tier"`
	Price       *int64    `json:"price,omitempty"`
}

// RateComponent is a rate component of a tariff.
type RateComponent struct {
	Href        string     `json:"href"`
	Description string     `json:"description,omitempty"`
	Intervals   []Interval `json:"intervals"`
}

// Tariff is a tariff profile.
type Tariff struct {
	Href            string          `json:"href"`
	Description     string          `json:"description,omitempty"`
	Currency        *uint16         `json:"currency,omitempty"`
	PriceMultiplier *int8           `json:"price_power_of_ten_multiplier,omitempty"`
	Primacy         uint8           `json:"primacy"`
	RateCode        string          `json:"rate_code,omitempty"`
	RateComponents  []RateComponent `json:"rate_components"`
}

// Pricing is the result of client pricing.
type Pricing struct {
	Tariffs []Tariff `json:"tariffs"`
}

func (p *Pricing) MarshalTextOutput() (string, error) {
	if len(p.Tariffs) == 0 {
		return "no tariffs", nil
	}
	var b strings.Builder
	for _, t := range p.Tariffs {
		fmt.Fprintf(&b, "%s  %s  primacy %d", t.Href, quote(t.Description), t.Primacy)
		if t.Currency != nil {
			fmt.Fprintf(&b, "  currency %d", *t.Currency)
		}
		b.WriteString("\n")
		for _, rc := range t.RateComponents {
			fmt.Fprintf(&b, "  %s  %s\n", rc.Href, quote(rc.Description))
			for _, iv := range rc.Intervals {
				price := "-"
				if iv.Price != nil {
					price = priceText(*iv.Price, t.PriceMultiplier)
				}
				line := fmt.Sprintf("    %s  %s for %s  tier %d  %s  %s", iv.Status, iv.Start.Format(time.RFC3339),
					time.Duration(iv.DurationS)*time.Second, iv.TOUTier, price, quote(iv.Description))
				b.WriteString(strings.TrimRight(line, " ") + "\n")
			}
		}
	}
	return b.String(), nil
}

func priceText(p int64, mult *int8) string {
	if mult == nil {
		return strconv.FormatInt(p, 10)
	}
	f := float64(p)
	for m := *mult; m < 0; m++ {
		f /= 10
	}
	for m := *mult; m > 0; m-- {
		f *= 10
	}
	return strconv.FormatFloat(f, 'f', -1, 64) + "/kWh"
}

func (p *Pricing) TableHeaders() []string {
	return []string{"TARIFF", "RATE_COMPONENT", "INTERVAL", "STATUS", "START", "DURATION_S", "PRICE"}
}
func (p *Pricing) TableRows() [][]string {
	rows := [][]string{}
	for _, t := range p.Tariffs {
		for _, rc := range t.RateComponents {
			for _, iv := range rc.Intervals {
				price := ""
				if iv.Price != nil {
					price = strconv.FormatInt(*iv.Price, 10)
				}
				rows = append(rows, []string{t.Description, rc.Description, iv.Href, iv.Status, iv.Start.Format(time.RFC3339),
					strconv.FormatUint(uint64(iv.DurationS), 10), price})
			}
		}
	}
	return rows
}

// ConnectionPoint is the result of client connection-point.
type ConnectionPoint struct {
	EndDevice string `json:"end_device"`
	Href      string `json:"href"`
	ID        string `json:"connection_point_id"`
	Set       bool   `json:"set"`
}

func (c *ConnectionPoint) MarshalTextOutput() (string, error) {
	if c.ID == "" {
		return fmt.Sprintf("connection point %s: not set", c.Href), nil
	}
	return fmt.Sprintf("connection point %s: %s", c.Href, c.ID), nil
}

func (c *ConnectionPoint) TableHeaders() []string { return []string{"END_DEVICE", "HREF", "ID"} }
func (c *ConnectionPoint) TableRows() [][]string  { return [][]string{{c.EndDevice, c.Href, c.ID}} }

// Event is a record of the streaming commands: client subscribe, client
// run and server start. Kind names what happened; the other fields are set
// as the kind has them.
type Event struct {
	Time     time.Time      `json:"time"`
	Kind     string         `json:"event"`
	Href     string         `json:"href,omitempty"`
	Resource string         `json:"resource,omitempty"`
	Status   string         `json:"status,omitempty"`
	MRID     string         `json:"mrid,omitempty"`
	Settings []ieee.Setting `json:"settings,omitempty"`
	Start    *time.Time     `json:"start,omitempty"`
	End      *time.Time     `json:"end,omitempty"`
	LFDI     string         `json:"lfdi,omitempty"`
	Address  string         `json:"address,omitempty"`
	Code     int            `json:"code,omitempty"`
	Message  string         `json:"message,omitempty"`
}

func (e *Event) MarshalTextOutput() (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %-14s", e.Time.Format("15:04:05.000"), e.Kind)
	for _, kv := range [][2]string{{"", e.Status}, {"", e.Href}, {"resource ", e.Resource}, {"mrid ", e.MRID},
		{"lfdi ", e.LFDI}, {"", e.Address}, {"", e.Message}} {
		if kv[1] != "" {
			b.WriteString(" " + kv[0] + kv[1])
		}
	}
	if e.Code != 0 {
		fmt.Fprintf(&b, " %d", e.Code)
	}
	if len(e.Settings) > 0 {
		b.WriteString(" " + settingsText(e.Settings))
	}
	if e.Start != nil && e.End != nil {
		fmt.Fprintf(&b, " %s..%s", e.Start.UTC().Format("15:04:05"), e.End.UTC().Format("15:04:05"))
	}
	return b.String(), nil
}

func (e *Event) TableHeaders() []string {
	return []string{"TIME", "EVENT", "STATUS", "HREF", "DETAIL"}
}
func (e *Event) TableRows() [][]string {
	detail := e.Message
	if len(e.Settings) > 0 {
		detail = strings.TrimSpace(detail + " " + settingsText(e.Settings))
	}
	if e.LFDI != "" {
		detail = strings.TrimSpace(detail + " lfdi " + e.LFDI)
	}
	if e.Address != "" {
		detail = strings.TrimSpace(detail + " " + e.Address)
	}
	href := e.Href
	if href == "" {
		href = e.Resource
	}
	status := e.Status
	if e.Code != 0 {
		status = strings.TrimSpace(status + " " + strconv.Itoa(e.Code))
	}
	return [][]string{{e.Time.Format("15:04:05.000"), e.Kind, status, href, detail}}
}
