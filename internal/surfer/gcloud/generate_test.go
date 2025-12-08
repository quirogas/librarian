// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcloud

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInferTrackFromPackage(t *testing.T) {
	for _, test := range []struct {
		name string
		pkg  string
		want string
	}{
		{
			name: "GA package",
			pkg:  "google.cloud.parallelstore.v1",
			want: "ga",
		},
		{
			name: "Beta package",
			pkg:  "google.cloud.parallelstore.v1beta",
			want: "beta",
		},
		{
			name: "Alpha package",
			pkg:  "google.cloud.parallelstore.v1alpha",
			want: "alpha",
		},
		{
			name: "Empty package",
			pkg:  "",
			want: "ga",
		},
		{
			name: "Package without version",
			pkg:  "google.cloud.parallelstore",
			want: "ga",
		},
		{
			name: "Other version",
			pkg:  "google.cloud.parallelstore.v2",
			want: "ga",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := inferTrackFromPackage(test.pkg)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetVerb(t *testing.T) {
	for _, test := range []struct {
		name       string
		methodName string
		want       string
	}{
		{"Get", "GetInstance", "describe"},
		{"List", "ListInstances", "list"},
		{"Create", "CreateInstance", "create"},
		{"Update", "UpdateInstance", "update"},
		{"Delete", "DeleteInstance", "delete"},
		{"Custom", "DetachDisk", "detach_disk"},
		{"Empty", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := getVerb(test.methodName)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetPluralFromPattern(t *testing.T) {
	for _, test := range []struct {
		name    string
		pattern string
		want    string
	}{
		{"Standard", "projects/{project}/locations/{location}/instances/{instance}", "instances"},
		{"Short", "shelves/{shelf}", "shelves"},
		{"No Variable End", "projects/{project}/locations", ""},
		{"Empty", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := getPluralFromPattern(test.pattern)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetSingularFromPattern(t *testing.T) {
	for _, test := range []struct {
		name    string
		pattern string
		want    string
	}{
		{"Standard", "projects/{project}/locations/{location}/instances/{instance}", "instance"},
		{"Short", "shelves/{shelf}", "shelf"},
		{"No Variable End", "projects/{project}/locations", ""},
		{"Empty", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := getSingularFromPattern(test.pattern)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetCollectionPathFromPattern(t *testing.T) {
	for _, test := range []struct {
		name    string
		pattern string
		want    string
	}{
		{"Standard", "projects/{project}/locations/{location}/instances/{instance}", "projects.locations.instances"},
		{"Short", "shelves/{shelf}", "shelves"},
		{"Root", "projects/{project}", "projects"},
		{"Mixed", "organizations/{organization}/locations/{location}/clusters/{cluster}", "organizations.locations.clusters"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := getCollectionPathFromPattern(test.pattern)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf(" mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

