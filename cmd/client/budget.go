// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"

	"github.com/otfabric/ieee20305ctl/internal/conn"
	"github.com/otfabric/ieee20305ctl/internal/errs"
)

// maxListed bounds the list items, and the documents read for them, that
// one command reads in all. The library bounds each list (MaxListItems);
// lists within lists (tariffs, rate components, intervals; programs,
// controls) multiply, so a server whose every list is long would keep a
// one-shot command going for the product of their lengths.
const maxListed = 5000

// errTooMany is the error of a command that would read more than maxListed.
func errTooMany(href string) error {
	return errs.New(errs.KindProtocol, errs.CodeInvalidResponse,
		fmt.Sprintf("%s: the server's lists hold more than %d items in all", href, maxListed), nil)
}

// count counts n documents read for a command, and fails beyond maxListed.
func count(d *conn.Device, href string, n int64) error {
	if d.Listed.Add(n) > maxListed {
		return errTooMany(href)
	}
	return nil
}

// iterate is Client.Iterate within the command's bound of maxListed.
func iterate(ctx context.Context, d *conn.Device, href string, pageSize uint32, fn func(item any) error) error {
	return d.Client.Iterate(ctx, href, pageSize, func(it any) error {
		if err := count(d, href, 1); err != nil {
			return err
		}
		return fn(it)
	})
}
