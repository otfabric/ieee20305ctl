// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/format"
	"github.com/spf13/cobra"
)

// Link is a link of a resource to another one.
type Link struct {
	// Name is the element name of the link, such as EndDeviceListLink.
	Name string `json:"name"`
	Href string `json:"href"`
	// All is the number of items of a list link, when the server says.
	All *uint32 `json:"all,omitempty"`
}

// Discovery is the result of client discover.
type Discovery struct {
	URL      string `json:"url"`
	PollRate uint32 `json:"pollRate"`
	Links    []Link `json:"links"`
}

func newDiscoverCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover server capabilities and resources",
		Long: `Connect to an IEEE 2030.5 server, retrieve its DeviceCapability resource,
and list the function sets it links to with their hrefs and, for lists,
their number of items.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := format.Parse(cmd.Flag("output").Value.String())
			if err != nil {
				return errs.WithCode(errs.ExitUsage, err)
			}
			d, err := discover(cmd.Context(), opts)
			if f == format.JSON {
				if err != nil {
					_ = format.WriteJSONError(cmd.OutOrStdout(), err)
					return err
				}
				return format.WriteJSON(cmd.OutOrStdout(), d)
			}
			if err != nil {
				return err
			}
			return printDiscovery(cmd.OutOrStdout(), d)
		},
	}
	format.AddFlag(cmd)
	return cmd
}

func discover(ctx context.Context, opts *options) (*Discovery, error) {
	c, _, err := opts.connect()
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	dc, err := c.DeviceCapability(ctx)
	if err != nil {
		return nil, classify(err)
	}
	u, err := c.Resolve(opts.dcap)
	if err != nil {
		return nil, classify(err)
	}
	return &Discovery{URL: u.String(), PollRate: uint32(dc.GetPollRate()), Links: links(dc)}, nil
}

var hrefType = reflect.TypeOf("")

// links returns the links of a generated resource type in schema order:
// every non-nil field that points to a struct with an Href.
func links(v any) []Link {
	var out []Link
	var walk func(reflect.Value)
	walk = func(rv reflect.Value) {
		rt := rv.Type()
		for i := 0; i < rt.NumField(); i++ {
			f, fv := rt.Field(i), rv.Field(i)
			if !f.IsExported() {
				continue
			}
			if f.Anonymous && fv.Kind() == reflect.Struct {
				walk(fv)
				continue
			}
			if fv.Kind() != reflect.Pointer || fv.IsNil() || fv.Elem().Kind() != reflect.Struct {
				continue
			}
			target := fv.Elem()
			href := target.FieldByName("Href")
			if !href.IsValid() || href.Type() != hrefType {
				continue
			}
			l := Link{Name: elementName(f), Href: href.String()}
			if all := target.FieldByName("All"); all.IsValid() && all.Kind() == reflect.Pointer && !all.IsNil() {
				n := uint32(all.Elem().Uint())
				l.All = &n
			}
			out = append(out, l)
		}
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	walk(rv)
	return out
}

// elementName returns the XML local name in the tag of a generated field.
func elementName(f reflect.StructField) string {
	tag, _, _ := strings.Cut(f.Tag.Get("xml"), ",")
	if i := strings.LastIndexByte(tag, ' '); i >= 0 {
		tag = tag[i+1:]
	}
	if tag == "" {
		return f.Name
	}
	return tag
}

func printDiscovery(w io.Writer, d *Discovery) error {
	if _, err := fmt.Fprintf(w, "DeviceCapability: %s\nPoll rate:        %d s\n\n", d.URL, d.PollRate); err != nil {
		return err
	}
	if len(d.Links) == 0 {
		_, err := fmt.Fprintln(w, "The server links no function sets.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "FUNCTION SET\tHREF\tITEMS")
	for _, l := range d.Links {
		items := ""
		if l.All != nil {
			items = fmt.Sprint(*l.All)
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", l.Name, l.Href, items)
	}
	return tw.Flush()
}
