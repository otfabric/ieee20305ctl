// SPDX-License-Identifier: MIT

// Package ieee holds what the commands of ieee20305ctl share about IEEE
// 2030.5 documents: the settings of a DERControlBase by their schema names,
// in both directions.
package ieee

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"

	ieee20305 "github.com/otfabric/go-ieee20305"
	"github.com/otfabric/go-ieee20305/csipaus"
)

// Setting is one control of a DERControlBase that is set.
type Setting struct {
	// Name is the schema element, such as opModMaxLimW or opModExpLimW.
	Name string `json:"name"`
	// Value is the value: a bool, a number (a percentage in hundredths of a
	// percent, as the schema has it), {value, multiplier} for a power, or the
	// href of a curve.
	Value any `json:"value"`
	// Text is the value for people: 50.00 %, 1500 W, true, /derp/1/dc/2.
	Text string `json:"text"`
}

// Power is a value with a power-of-ten multiplier, such as an ActivePower.
type Power struct {
	Value      int64 `json:"value"`
	Multiplier int64 `json:"multiplier"`
}

// Settings returns the controls of b that are set, in schema-name order.
// b is a *ieee20305.DERControlBase or a *csipaus.DERControlBase.
func Settings(b any) []Setting {
	out := []Setting{}
	walk(reflect.ValueOf(b), func(name string, f reflect.Value) {
		if s, ok := setting(name, f); ok {
			out = append(out, s)
		}
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CSIPAUSOnly returns the names of the settings of b that CSIP-AUS adds to
// IEEE 2030.5: the site limits.
func CSIPAUSOnly(b *csipaus.DERControlBase) []string {
	var out []string
	for _, s := range Settings(&csipaus.DERControlBase{OpModExpLimW: b.OpModExpLimW, OpModImpLimW: b.OpModImpLimW,
		OpModGenLimW: b.OpModGenLimW, OpModLoadLimW: b.OpModLoadLimW, OpModStorageTargetW: b.OpModStorageTargetW}) {
		out = append(out, s.Name)
	}
	return out
}

// Describe is the Settings of b on one line: opModConnect=true opModMaxLimW=50.00 %.
func Describe(b any) string {
	var parts []string
	for _, s := range Settings(b) {
		parts = append(parts, s.Name+"="+s.Text)
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, " ")
}

// Names returns the names of the settings ParseSettings knows.
func Names() []string {
	var names []string
	walk(reflect.ValueOf(&csipaus.DERControlBase{}), func(name string, f reflect.Value) {
		if settable(f.Type()) {
			names = append(names, name)
		}
	})
	sort.Strings(names)
	return names
}

// ParseSettings sets the controls of name=value pairs on a CSIP-AUS
// DERControlBase (whose IEEE 2030.5 part serves the base standard). Values:
// true or false; a percentage (50 or 50.25) for the PerCent controls; watts,
// vars or volts (1500, -2000) for the controls of a power, which choose
// the multiplier that fits.
func ParseSettings(pairs []string) (csipaus.DERControlBase, error) {
	var b csipaus.DERControlBase
	fields := map[string]reflect.Value{}
	walk(reflect.ValueOf(&b), func(name string, f reflect.Value) { fields[strings.ToLower(name)] = f })
	for _, p := range pairs {
		name, value, ok := strings.Cut(p, "=")
		if !ok {
			return b, fmt.Errorf("%q: a setting is name=value, such as opModMaxLimW=50", p)
		}
		f, ok := fields[strings.ToLower(strings.TrimSpace(name))]
		if !ok || !settable(f.Type()) {
			return b, fmt.Errorf("%q: not a control of DERControlBase this tool sets (see --help)", name)
		}
		if err := set(f, strings.TrimSpace(value)); err != nil {
			return b, fmt.Errorf("%s: %w", name, err)
		}
	}
	return b, nil
}

// walk calls fn for each control field of the DERControlBase v points to,
// with its schema name, descending into the embedded IEEE 2030.5 base.
func walk(v reflect.Value, fn func(name string, f reflect.Value)) {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.Anonymous {
			walk(v.Field(i).Addr(), fn)
			continue
		}
		tag := sf.Tag.Get("xml")
		if tag == "" || strings.HasPrefix(tag, ",") {
			continue
		}
		name := tag[strings.LastIndex(tag, " ")+1:]
		name, _, _ = strings.Cut(name, ",")
		if !strings.HasPrefix(name, "opMod") && !strings.HasPrefix(name, "ramp") {
			continue
		}
		fn(name, v.Field(i))
	}
}

// fieldsOf finds the Value, Multiplier and Href fields of a control type,
// in it or its embedded structs.
func fieldsOf(v reflect.Value) (value, mult, href reflect.Value) {
	var visit func(v reflect.Value)
	visit = func(v reflect.Value) {
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			sf := t.Field(i)
			switch {
			case sf.Anonymous && sf.Type.Kind() == reflect.Struct:
				visit(v.Field(i))
			case sf.Name == "Value" && !value.IsValid():
				value = v.Field(i)
			case sf.Name == "Multiplier" && !mult.IsValid():
				mult = v.Field(i)
			case sf.Name == "Href" && !href.IsValid():
				href = v.Field(i)
			}
		}
	}
	visit(v)
	return
}

func setting(name string, f reflect.Value) (Setting, bool) {
	if f.Kind() != reflect.Pointer || f.IsNil() {
		return Setting{}, false
	}
	e := f.Elem()
	if e.Kind() == reflect.Bool {
		return Setting{Name: name, Value: e.Bool(), Text: strconv.FormatBool(e.Bool())}, true
	}
	if e.Kind() == reflect.Uint32 || e.Kind() == reflect.Uint16 { // rampTms and friends
		return Setting{Name: name, Value: e.Uint(), Text: strconv.FormatUint(e.Uint(), 10)}, true
	}
	if e.Kind() != reflect.Struct {
		return Setting{Name: name, Value: fmt.Sprint(e.Interface()), Text: fmt.Sprint(e.Interface())}, true
	}
	value, mult, href := fieldsOf(e)
	switch {
	case href.IsValid():
		return Setting{Name: name, Value: href.String(), Text: href.String()}, true
	case value.IsValid() && mult.IsValid():
		p := Power{Value: intOf(value), Multiplier: intOf(mult)}
		return Setting{Name: name, Value: p, Text: powerText(name, p)}, true
	case value.IsValid():
		n := intOf(value)
		if percent(e.Type()) {
			return Setting{Name: name, Value: n, Text: fmt.Sprintf("%.2f %%", float64(n)/100)}, true
		}
		return Setting{Name: name, Value: n, Text: strconv.FormatInt(n, 10)}, true
	}
	return Setting{}, false
}

func percent(t reflect.Type) bool { return strings.Contains(t.Name(), "PerCent") }

func powerText(name string, p Power) string {
	unit := "W"
	switch {
	case strings.Contains(name, "Var"):
		unit = "var"
	case strings.HasSuffix(name, "V"):
		unit = "V"
	}
	v := float64(p.Value) * math.Pow10(int(p.Multiplier))
	return strconv.FormatFloat(v, 'f', -1, 64) + " " + unit
}

func intOf(v reflect.Value) int64 {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(v.Uint())
	default:
		return 0
	}
}

// settable reports whether set can set a field of type t.
func settable(t reflect.Type) bool {
	if t.Kind() != reflect.Pointer {
		return false
	}
	e := t.Elem()
	if e.Kind() == reflect.Bool {
		return true
	}
	if e.Kind() != reflect.Struct {
		return false
	}
	value, _, href := fieldsOf(reflect.New(e).Elem())
	return value.IsValid() && !href.IsValid() && value.Kind() != reflect.Struct
}

func set(f reflect.Value, s string) error {
	e := reflect.New(f.Type().Elem())
	if e.Elem().Kind() == reflect.Bool {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return fmt.Errorf("%q is not true or false", s)
		}
		e.Elem().SetBool(b)
		f.Set(e)
		return nil
	}
	value, mult, _ := fieldsOf(e.Elem())
	x, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("%q is not a number", s)
	}
	if !mult.IsValid() {
		if percent(e.Elem().Type()) {
			x *= 100 // hundredths of a percent
		}
		if x != math.Trunc(x) {
			return fmt.Errorf("%s has more precision than the control holds", s)
		}
		if err := setInt(value, int64(x)); err != nil {
			return err
		}
		f.Set(e)
		return nil
	}
	// A power: the smallest multiplier whose value fits.
	m := int64(0)
	for math.Abs(x) > 32767 {
		x /= 10
		m++
	}
	for x != math.Trunc(x) && m > -3 {
		x *= 10
		m--
	}
	if x != math.Trunc(x) || math.Abs(x) > 32767 {
		return fmt.Errorf("%s does not fit a value with a power-of-ten multiplier", s)
	}
	if err := setInt(value, int64(x)); err != nil {
		return err
	}
	mult.SetInt(m)
	f.Set(e)
	return nil
}

func setInt(v reflect.Value, n int64) error {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.OverflowInt(n) {
			return fmt.Errorf("%d is out of range", n)
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if n < 0 || v.OverflowUint(uint64(n)) {
			return fmt.Errorf("%d is out of range", n)
		}
		v.SetUint(uint64(n))
	default:
		return fmt.Errorf("a value of kind %s", v.Kind())
	}
	return nil
}

// BaseOf returns the CSIP-AUS DERControlBase of a control document: a
// DERControl or DefaultDERControl of either package. The IEEE 2030.5 part
// of a base control is the whole of it.
func BaseOf(v any) (*csipaus.DERControlBase, bool) {
	switch x := v.(type) {
	case *ieee20305.DERControl:
		return &csipaus.DERControlBase{DERControlBase: x.DERControlBase}, true
	case *ieee20305.DefaultDERControl:
		return &csipaus.DERControlBase{DERControlBase: x.DERControlBase}, true
	case *csipaus.DERControl:
		return &x.DERControlBase, true
	case *csipaus.DefaultDERControl:
		return &x.DERControlBase, true
	}
	return nil, false
}
