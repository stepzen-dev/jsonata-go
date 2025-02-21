// Copyright 2018 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package jlib_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/stepzen-dev/jsonata-go/jlib"
	"github.com/stepzen-dev/jsonata-go/jtypes"
)

func TestFromMillis(t *testing.T) {

	date := time.Date(2018, time.September, 30, 15, 58, 5, int(762*time.Millisecond), time.UTC)
	input := date.UnixNano() / int64(time.Millisecond)

	data := []struct {
		Picture       string
		TZ            string
		Output        string
		ExpectedError bool
	}{
		{
			Picture: "[Y0001]-[M01]-[D01]",
			Output:  "2018-09-30",
		},
		/*{
			Picture: "[[[Y0001]-[M01]-[D01]]]",
			Output:  "[2018-09-30]",
		},*/
		{
			Picture: "[M]-[D]-[Y]",
			Output:  "9-30-2018",
		},
		{
			Picture: "[D1o] [MNn], [Y]",
			Output:  "30th September, 2018",
		},
		{
			Picture: "[D01] [MN,*-3] [Y0001]",
			Output:  "30 SEP 2018",
		},
		{
			Picture: "[h]:[m01] [PN]",
			Output:  "3:58 PM",
		},
		{
			Picture: "[h]:[m01]:[s01] [Pn]",
			Output:  "3:58:05 pm",
		},
		{
			Picture: "[h]:[m01]:[s01] [PN] [ZN,*-3]",
			Output:  "3:58:05 PM UTC",
		},
		{
			Picture: "[h]:[m01]:[s01] o'clock [PN] [ZN,*-3]",
			Output:  "3:58:05 o'clock PM UTC",
		},
		{
			Picture: "[H01]:[m01]:[s01].[f001]",
			Output:  "15:58:05.762",
		},
		{
			Picture: "[H01]:[m01]:[s01] [Z]",
			TZ:      "+0200",
			Output:  "17:58:05 +02:00",
		},
		{
			Picture: "[H01]:[m01]:[s01] [z]",
			TZ:      "-0500",
			Output:  "10:58:05 GMT-05:00",
		},
		{
			Picture: "[H01]:[m01]:[s01] [z]",
			TZ:      "-0630",
			Output:  "09:28:05 GMT-06:30",
		},
		{
			Picture: "[H01]:[m01]:[s01] [z]",
			// Invalid TZ
			TZ:            "-0",
			ExpectedError: true,
		},
		{
			Picture: "[h].[m01][Pn] on [FNn], [D1o] [MNn]",
			Output:  "3.58pm on Sunday, 30th September",
		},
		{
			Picture: "[M01]/[D01]/[Y0001] at [H01]:[m01]:[s01]",
			Output:  "09/30/2018 at 15:58:05",
		},
	}

	for _, test := range data {

		var picture jtypes.OptionalString
		var tz jtypes.OptionalString

		if test.Picture != "" {
			picture.Set(reflect.ValueOf(test.Picture))
		}

		if test.TZ != "" {
			tz.Set(reflect.ValueOf(test.TZ))
		}

		got, err := jlib.FromMillis(input, picture, tz)

		if test.ExpectedError && err == nil {
			t.Errorf("%s: Expected error, got nil", test.Picture)
		} else if got != test.Output {
			t.Errorf("%s: Expected %q, got %q", test.Picture, test.Output, got)
		}
	}
}
