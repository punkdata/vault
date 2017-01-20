package vault

import (
	"reflect"
	"testing"

	"github.com/hashicorp/vault/logical"
)

func TestACL_Capabilities(t *testing.T) {
	// Create the root policy ACL
	policy := []*Policy{&Policy{Name: "root"}}
	acl, err := NewACL(policy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	actual := acl.Capabilities("any/path")
	expected := []string{"root"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: got\n%#v\nexpected\n%#v\n", actual, expected)
	}

	policies, err := Parse(aclPolicy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	acl, err = NewACL([]*Policy{policies})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	actual = acl.Capabilities("dev")
	expected = []string{"deny"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: path:%s\ngot\n%#v\nexpected\n%#v\n", "deny", actual, expected)
	}

	actual = acl.Capabilities("dev/")
	expected = []string{"sudo", "read", "list", "update", "delete", "create"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: path:%s\ngot\n%#v\nexpected\n%#v\n", "dev/", actual, expected)
	}

	actual = acl.Capabilities("stage/aws/test")
	expected = []string{"sudo", "read", "list", "update"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: path:%s\ngot\n%#v\nexpected\n%#v\n", "stage/aws/test", actual, expected)
	}

}

func TestACL_Root(t *testing.T) {
	// Create the root policy ACL
	policy := []*Policy{&Policy{Name: "root"}}
	acl, err := NewACL(policy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	request := new(logical.Request)
	request.Operation = logical.UpdateOperation
	request.Path = "sys/mount/foo"
	allowed, rootPrivs := acl.AllowOperation(request)
	if !rootPrivs {
		t.Fatalf("expected root")
	}
	if !allowed {
		t.Fatalf("expected permissions")
	}
}

func TestACL_Single(t *testing.T) {
	policy, err := Parse(aclPolicy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	acl, err := NewACL([]*Policy{policy})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Type of operation is not important here as we only care about checking
	// sudo/root
	request := new(logical.Request)
	request.Operation = logical.ReadOperation
	request.Path = "sys/mount/foo"
	_, rootPrivs := acl.AllowOperation(request)
	if rootPrivs {
		t.Fatalf("unexpected root")
	}

	type tcase struct {
		op        logical.Operation
		path      string
		allowed   bool
		rootPrivs bool
	}
	tcases := []tcase{
		{logical.ReadOperation, "root", false, false},
		{logical.HelpOperation, "root", true, false},

		{logical.ReadOperation, "dev/foo", true, true},
		{logical.UpdateOperation, "dev/foo", true, true},

		{logical.DeleteOperation, "stage/foo", true, false},
		{logical.ListOperation, "stage/aws/foo", true, true},
		{logical.UpdateOperation, "stage/aws/foo", true, true},
		{logical.UpdateOperation, "stage/aws/policy/foo", true, true},

		{logical.DeleteOperation, "prod/foo", false, false},
		{logical.UpdateOperation, "prod/foo", false, false},
		{logical.ReadOperation, "prod/foo", true, false},
		{logical.ListOperation, "prod/foo", true, false},
		{logical.ReadOperation, "prod/aws/foo", false, false},

		{logical.ReadOperation, "foo/bar", true, true},
		{logical.ListOperation, "foo/bar", false, true},
		{logical.UpdateOperation, "foo/bar", false, true},
		{logical.CreateOperation, "foo/bar", true, true},
	}

	for _, tc := range tcases {
		request := new(logical.Request)
		request.Operation = tc.op
		request.Path = tc.path
		allowed, rootPrivs := acl.AllowOperation(request)
		if allowed != tc.allowed {
			t.Fatalf("bad: case %#v: %v, %v", tc, allowed, rootPrivs)
		}
		if rootPrivs != tc.rootPrivs {
			t.Fatalf("bad: case %#v: %v, %v", tc, allowed, rootPrivs)
		}
	}
}

func TestACL_Layered(t *testing.T) {
	policy1, err := Parse(aclPolicy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	policy2, err := Parse(aclPolicy2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	acl, err := NewACL([]*Policy{policy1, policy2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	testLayeredACL(t, acl)
}

func testLayeredACL(t *testing.T, acl *ACL) {
	// Type of operation is not important here as we only care about checking
	// sudo/root
	request := new(logical.Request)
	request.Operation = logical.ReadOperation
	request.Path = "sys/mount/foo"
	_, rootPrivs := acl.AllowOperation(request)
	if rootPrivs {
		t.Fatalf("unexpected root")
	}

	type tcase struct {
		op        logical.Operation
		path      string
		allowed   bool
		rootPrivs bool
	}
	tcases := []tcase{
		{logical.ReadOperation, "root", false, false},
		{logical.HelpOperation, "root", true, false},

		{logical.ReadOperation, "dev/foo", true, true},
		{logical.UpdateOperation, "dev/foo", true, true},
		{logical.ReadOperation, "dev/hide/foo", false, false},
		{logical.UpdateOperation, "dev/hide/foo", false, false},

		{logical.DeleteOperation, "stage/foo", true, false},
		{logical.ListOperation, "stage/aws/foo", true, true},
		{logical.UpdateOperation, "stage/aws/foo", true, true},
		{logical.UpdateOperation, "stage/aws/policy/foo", false, false},

		{logical.DeleteOperation, "prod/foo", true, false},
		{logical.UpdateOperation, "prod/foo", true, false},
		{logical.ReadOperation, "prod/foo", true, false},
		{logical.ListOperation, "prod/foo", true, false},
		{logical.ReadOperation, "prod/aws/foo", false, false},

		{logical.ReadOperation, "sys/status", false, false},
		{logical.UpdateOperation, "sys/seal", true, true},

		{logical.ReadOperation, "foo/bar", false, false},
		{logical.ListOperation, "foo/bar", false, false},
		{logical.UpdateOperation, "foo/bar", false, false},
		{logical.CreateOperation, "foo/bar", false, false},
	}

	for _, tc := range tcases {
		request := new(logical.Request)
		request.Operation = tc.op
		request.Path = tc.path
		allowed, rootPrivs := acl.AllowOperation(request)
		if allowed != tc.allowed {
			t.Fatalf("bad: case %#v: %v, %v", tc, allowed, rootPrivs)
		}
		if rootPrivs != tc.rootPrivs {
			t.Fatalf("bad: case %#v: %v, %v", tc, allowed, rootPrivs)
		}
	}
}

func TestPolicyMerge(t *testing.T) {
	policy, err := Parse(mergingPolicies)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	acl, err := NewACL([]*Policy{policy})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	type tcase struct {
		path    string
		allowed map[string][]interface{}
		denied  map[string][]interface{}
	}

	tcases := []tcase{
		{"foo/bar", nil, map[string][]interface{}{"zip": []interface{}{}, "baz": []interface{}{}}},
		{"hello/universe", map[string][]interface{}{"foo": []interface{}{}, "bar": []interface{}{}}, nil},
		{"allow/all", map[string][]interface{}{"*": []interface{}{}}, nil},
		{"allow/all1", map[string][]interface{}{"*": []interface{}{}}, nil},
		{"deny/all", nil, map[string][]interface{}{"*": []interface{}{}}},
		{"deny/all1", nil, map[string][]interface{}{"*": []interface{}{}}},
		{"value/merge", map[string][]interface{}{"test": []interface{}{1, 2, 3, 4}}, map[string][]interface{}{"test": []interface{}{1, 2, 3, 4}}},
	}

	for _, tc := range tcases {
		raw, ok := acl.exactRules.Get(tc.path)
		if !ok {
			t.Fatalf("Could not find acl entry for path %s", tc.path)
		}

		p := raw.(*Permissions)
		if !reflect.DeepEqual(tc.allowed, p.AllowedParameters) {
			t.Fatalf("Allowed paramaters did not match, Expected: %#v, Got: %#v", tc.allowed, p.AllowedParameters)
		}
		if !reflect.DeepEqual(tc.denied, p.DeniedParameters) {
			t.Fatalf("Denied paramaters did not match, Expected: %#v, Got: %#v", tc.denied, p.DeniedParameters)
		}
	}
}

func TestAllowOperation(t *testing.T) {
	policy, err := Parse(permissionsPolicy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	acl, err := NewACL([]*Policy{policy})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	toperations := []logical.Operation{
		logical.UpdateOperation,
		logical.DeleteOperation,
		logical.CreateOperation,
	}
	type tcase struct {
		path       string
		parameters []string
		allowed    bool
	}

	tcases := []tcase{
		{"dev/ops", []string{"zip"}, true},
		{"foo/bar", []string{"zap"}, false},
		{"foo/baz", []string{"hello"}, true},
		{"foo/baz", []string{"zap"}, false},
		{"broken/phone", []string{"steve"}, false},
		{"hello/world", []string{"one"}, false},
		{"tree/fort", []string{"one"}, true},
		{"tree/fort", []string{"beer"}, false},
		{"fruit/apple", []string{"pear"}, false},
		{"fruit/apple", []string{"one"}, false},
		{"cold/weather", []string{"four"}, true},
		{"var/aws", []string{"cold", "warm", "kitty"}, false},
	}

	for _, tc := range tcases {
		request := logical.Request{Path: tc.path, Data: make(map[string]interface{})}
		for _, parameter := range tc.parameters {
			request.Data[parameter] = ""
		}
		for _, op := range toperations {
			request.Operation = op
			allowed, _ := acl.AllowOperation(&request)
			if allowed != tc.allowed {
				t.Fatalf("bad: case %#v: %v", tc, allowed)
			}
		}
	}
}

func TestValuePermissions(t *testing.T) {
	policy, err := Parse(valuePermissionsPolicy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	acl, err := NewACL([]*Policy{policy})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	toperations := []logical.Operation{
		logical.UpdateOperation,
		logical.DeleteOperation,
		logical.CreateOperation,
	}
	type tcase struct {
		path       string
		parameters []string
		values     []interface{}
		allowed    bool
	}

	tcases := []tcase{
		{"dev/ops", []string{"allow"}, []interface{}{"good"}, true},
		{"dev/ops", []string{"allow"}, []interface{}{"bad"}, false},
		{"foo/bar", []string{"deny"}, []interface{}{"bad"}, false},
		{"foo/bar", []string{"deny"}, []interface{}{"good"}, true},
		{"foo/bar", []string{"allow"}, []interface{}{"good"}, true},
		{"foo/baz", []string{"allow"}, []interface{}{"good"}, true},
		{"foo/baz", []string{"deny"}, []interface{}{"bad"}, false},
		{"foo/baz", []string{"deny"}, []interface{}{"good"}, true},
		{"foo/baz", []string{"allow"}, []interface{}{"bad"}, false},
		{"foo/baz", []string{"neither"}, []interface{}{"bad"}, false},
		{"fizz/buzz", []string{"allow_multi"}, []interface{}{"good"}, true},
		{"fizz/buzz", []string{"allow_multi"}, []interface{}{"good1"}, true},
		{"fizz/buzz", []string{"allow_multi"}, []interface{}{"good2"}, true},
		{"fizz/buzz", []string{"allow_multi"}, []interface{}{"bad"}, false},
		{"fizz/buzz", []string{"allow_multi"}, []interface{}{"bad"}, false},
		{"fizz/buzz", []string{"allow_multi", "allow"}, []interface{}{"good1", "good"}, true},
		{"fizz/buzz", []string{"deny_multi"}, []interface{}{"bad2"}, false},
		{"fizz/buzz", []string{"deny_multi", "allow_multi"}, []interface{}{"good", "good2"}, true},
		//	{"test/types", []string{"array"}, []interface{}{[1]string{"good"}}, true},
		{"test/types", []string{"map"}, []interface{}{map[string]interface{}{"good": "one"}}, true},
		{"test/types", []string{"map"}, []interface{}{map[string]interface{}{"bad": "one"}}, false},
		{"test/types", []string{"int"}, []interface{}{1}, true},
		{"test/types", []string{"int"}, []interface{}{3}, false},
		{"test/types", []string{"bool"}, []interface{}{false}, false},
		{"test/types", []string{"bool"}, []interface{}{true}, true},
	}

	for _, tc := range tcases {
		request := logical.Request{Path: tc.path, Data: make(map[string]interface{})}
		for i, parameter := range tc.parameters {
			request.Data[parameter] = tc.values[i]
		}
		for _, op := range toperations {
			request.Operation = op
			allowed, _ := acl.AllowOperation(&request)
			if allowed != tc.allowed {
				t.Fatalf("bad: case %#v: %v", tc, allowed)
			}
		}
	}
}

var tokenCreationPolicy = `
name = "tokenCreation"
path "auth/token/create*" {
	capabilities = ["update", "create", "sudo"]
}
`

var aclPolicy = `
name = "dev"
path "dev/*" {
	policy = "sudo"
}
path "stage/*" {
	policy = "write"
}
path "stage/aws/*" {
	policy = "read"
	capabilities = ["update", "sudo"]
}
path "stage/aws/policy/*" {
	policy = "sudo"
}
path "prod/*" {
	policy = "read"
}
path "prod/aws/*" {
	policy = "deny"
}
path "sys/*" {
	policy = "deny"
}
path "foo/bar" {
	capabilities = ["read", "create", "sudo"]
}
`

var aclPolicy2 = `
name = "ops"
path "dev/hide/*" {
	policy = "deny"
}
path "stage/aws/policy/*" {
	policy = "deny"
	# This should have no effect
	capabilities = ["read", "update", "sudo"]
}
path "prod/*" {
	policy = "write"
}
path "sys/seal" {
	policy = "sudo"
}
path "foo/bar" {
	capabilities = ["deny"]
}
`

//test merging
var mergingPolicies = `
name = "ops"
path "foo/bar" {
	policy = "write"
	permissions = {
		denied_parameters = {
			"baz" = []
		}
	}
}
path "foo/bar" {
	policy = "write"
	permissions = {
		denied_parameters = {
			"zip" = []
		}
	}
}
path "hello/universe" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"foo" = []
		}
	}
}
path "hello/universe" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"bar" = []
		}
  }
}
path "allow/all" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"test" = []
		}
	}
}
path "allow/all" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"*" = []
		}
  }
}
path "allow/all1" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"*" = []
		}
  }
}
path "allow/all1" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"test" = []
		}
  }
}
path "deny/all" {
	policy = "write"
	permissions = {
		denied_parameters = {
			"frank" = []
		}
	}
}
path "deny/all" {
	policy = "write"
	permissions = {
		denied_parameters = {
			"*" = []
		}
  }
}
path "deny/all1" {
	policy = "write"
	permissions = {
		denied_parameters = {
			"*" = []
		}
  }
}
path "deny/all1" {
	policy = "write"
	permissions = {
		denied_parameters = {
			"test" = []
		}
  }
}
path "value/merge" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"test" = [1, 2]
		}
		denied_parameters = {
			"test" = [1, 2]
		}

	}
}
path "value/merge" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"test" = [3, 4]
		}
		denied_parameters = {
			"test" = [3, 4]
		}
  }
}
`

//allow operation testing
var permissionsPolicy = `
name = "dev"
path "dev/*" {
	policy = "write"
	
  permissions = {
  	allowed_parameters = {
  		"zip" = []
  	}
  }
}
path "foo/bar" {
	policy = "write"
	permissions = {
		denied_parameters = {
			"zap" = []
		}
  }
}
path "foo/baz" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"hello" = []
		}
		denied_parameters = {
			"zap" = []
		}
  }
}
path "broken/phone" {
	policy = "write"
	permissions = {
		allowed_parameters = {
		  "steve" = []
		}
		denied_parameters = {
		  "steve" = []
		}
	}
}
path "hello/world" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"*" = []
		}
		denied_parameters = {
			"*" = []
		}
  }
}
path "tree/fort" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"*" = []
		}
		denied_parameters = {
			"beer" = []
		}
  }
}
path "fruit/apple" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"pear" = []
		}
		denied_parameters = {
			"*" = []
		}
  }
}
path "cold/weather" {
	policy = "write"
	permissions = {
		allowed_parameters = {}
		denied_parameters = {}
	}
}
path "var/aws" {
  	policy = "write"
	permissions = {
	  	allowed_parameters = {
			"*" = []
		}
		denied_parameters = {
		  	"soft" = []
			"warm" = []
			"kitty" = []
		}
	}
}
`

//allow operation testing
var valuePermissionsPolicy = `
name = "op"
path "dev/*" {
	policy = "write"
	
	permissions = {
		allowed_parameters = {
  			"allow" = ["good"]
	  	}
	}
}
path "foo/bar" {
	policy = "write"
	permissions = {
		denied_parameters = {
			"deny" = ["bad"]
		}
	}
}
path "foo/baz" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"allow" = ["good"]
		}
		denied_parameters = {
			"deny" = ["bad"]
		}
	}
}
path "fizz/buzz" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"allow_multi" = ["good", "good1", "good2"]
			"allow" = ["good"]
		}
		denied_parameters = {
			"deny_multi" = ["bad", "bad1", "bad2"]
		}
	}
}
path "test/types" {
	policy = "write"
	permissions = {
		allowed_parameters = {
			"map" = [{"good" = "one"}]
			"int" = [1, 2]
		}
		denied_parameters = {
			"bool" = [false]
		}
	}
}
`
