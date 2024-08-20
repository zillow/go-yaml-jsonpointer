// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: BSD-2-Clause

package yptr_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	yptr "github.com/zillow/go-yaml-jsonpointer"
	"github.com/zillow/go-yaml/v3"
)

func ExampleInsert() {
	src := `
d:
  - e
  - f:
      g: x
  - h: y
  - - i
    - j
`
	arr1 := `[1, 2, 3]`
	map1 := `q: xyz`
	s1 := `x`

	var n, a, m, x yaml.Node
	yaml.Unmarshal([]byte(src), &n)
	yaml.Unmarshal([]byte(arr1), &a)
	yaml.Unmarshal([]byte(map1), &m)
	yaml.Unmarshal([]byte(s1), &x)

	_ = yptr.Insert(&n, `/f/d`, a)
	_ = yptr.Insert(&n, ``, m)

	_ = yptr.Insert(&n, `/d/1`, m)
	_ = yptr.Insert(&n, `/d/-/c`, x)
	_ = yptr.Insert(&n, `/d/2/f`, m)
	_ = yptr.Insert(&n, `/d/3/f`, a)
	_ = yptr.Insert(&n, `/d/4/-`, x)

	out, err := yaml.Marshal(n.Content[0])
	if err != nil {
		panic(err)
	}

	fmt.Println(string(out))
	/* Output:
d:
    - e
    - q: xyz
    - f:
        g: x
        q: xyz
    - h: y
      f: [1, 2, 3]
    - - i
      - j
      - x
    - c: x
f:
    d: [1, 2, 3]
q: xyz
*/
}

func TestInsertErrors(t *testing.T) {
	src := `
a:
  b:
    c: 42
d:
- e
- f
`
	s1 := `x`
	var n, x yaml.Node
	yaml.Unmarshal([]byte(src), &n)
	yaml.Unmarshal([]byte(s1), &x)

	tests := []struct {
		ptr string
		value yaml.Node
		err string
	}{
		{``, x, "cannot insert node type"},
		{`/a/b/c`, x, "cannot insert node type"},
		{`/d`, x, "cannot insert node type"},
		{`/a/b/c/f`, x, "unhandled node type"},
		{`/d/f`, x, "strconv.Atoi"},
		{`/d/5`, x, "out of bounds"},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			err := yptr.Insert(&n, tc.ptr, tc.value)
			if err == nil {
				t.Fatal("expecting error")
			}
			if !strings.HasPrefix(err.Error(), tc.err) {
				t.Fatalf("expecting error %q, got %q", tc.err, err)
			}
		})
	}
}

func ExampleFind() {
	src := `
a:
  b:
    c: 42
d:
- e
- f
`
	var n yaml.Node
	yaml.Unmarshal([]byte(src), &n)

	r, _ := yptr.Find(&n, `/a/b/c`)
	fmt.Printf("Scalar %q at %d:%d\n", r.Value, r.Line, r.Column)

	r, _ = yptr.Find(&n, `/d/0`)
	fmt.Printf("Scalar %q at %d:%d\n", r.Value, r.Line, r.Column)
	// Output: Scalar "42" at 4:8
	// Scalar "e" at 6:3
}

func ExampleFind_extension() {
	src := `kind: Deployment
apiVersion: apps/v1
metadata:
  name: foo
spec:
  template:
    spec:
      replicas: 1
      containers:
      - name: app
        image: nginx
      - name: sidecar
        image: mysidecar
`
	var n yaml.Node
	yaml.Unmarshal([]byte(src), &n)

	r, _ := yptr.Find(&n, `/spec/template/spec/containers/1/image`)
	fmt.Printf("Scalar %q at %d:%d\n", r.Value, r.Line, r.Column)

	r, _ = yptr.Find(&n, `/spec/template/spec/containers/~{"name":"app"}/image`)
	fmt.Printf("Scalar %q at %d:%d\n", r.Value, r.Line, r.Column)

	// Output: Scalar "mysidecar" at 13:16
	// Scalar "nginx" at 11:16
}

func ExampleFind_jsonPointerCompat() {
	// the array item match syntax doesn't accidentally match a field that just happens
	// to contain the same characters.
	src := `a:
  "{\"b\":\"c\"}": d
`
	var n yaml.Node
	yaml.Unmarshal([]byte(src), &n)

	r, _ := yptr.Find(&n, `/a/{"b":"c"}`)

	fmt.Printf("Scalar %q at %d:%d\n", r.Value, r.Line, r.Column)

	// Output: Scalar "d" at 2:20
}

func TestParse(t *testing.T) {
	src := `
spec:
  template:
    spec:
      replicas: 1
      containers:
      - name: app
        image: nginx
`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		t.Fatal(err)
	}
	if _, err := yptr.Find(&root, "/bad/path"); !errors.Is(err, yptr.ErrNotFound) {
		t.Fatalf("expecting not found error, got: %v", err)
	}

	testCases := []struct {
		ptr    string
		value  string
		line   int
		column int
	}{
		{`/spec/template/spec/replicas`, "1", 5, 17},
		{`/spec/template/spec/containers/0/image`, "nginx", 8, 16},
		{`/spec/template/spec/containers/~{"name":"app"}/image`, "nginx", 8, 16},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			r, err := yptr.Find(&root, tc.ptr)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := r.Value, tc.value; got != want {
				t.Fatalf("got: %v, want: %v", got, want)
			}
			if got, want := r.Line, tc.line; got != want {
				t.Errorf("got: %v, want: %v", got, want)
			}
			if got, want := r.Column, tc.column; got != want {
				t.Errorf("got: %v, want: %v", got, want)
			}
		})
	}

	errorCases := []struct {
		ptr string
		err error
	}{
		{"a", fmt.Errorf(`JSON pointer must be empty or start with a "/`)},
		{"/a", yptr.ErrNotFound},
	}
	for i, tc := range errorCases {
		t.Run(fmt.Sprint("error", i), func(t *testing.T) {
			_, err := yptr.Find(&root, tc.ptr)
			if err == nil {
				t.Fatal("error expected")
			}
			if got, want := err, tc.err; got.Error() != want.Error() && !errors.Is(got, want) {
				t.Errorf("got: %v, want: %v", got, want)
			}
		})
	}
}
