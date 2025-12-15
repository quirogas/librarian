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
	"github.com/googleapis/librarian/internal/sidekick/api"
)

func lit(s string) api.PathSegment {
	return api.PathSegment{Literal: &s}
}

func variable(name string) api.PathSegment {
	return api.PathSegment{Variable: api.NewPathVariable(name).WithMatch()}
}

func TestInferTrackFromPackage(t *testing.T) {
	for _, test := range []struct {
		name string
		pkg  string
		want string
	}{
		{"GA package", "google.cloud.parallelstore.v1", "ga"},
		{"Beta package", "google.cloud.parallelstore.v1beta", "beta"},
		{"Alpha package", "google.cloud.parallelstore.v1alpha", "alpha"},
		{"Empty package", "", "ga"},
		{"Package without version", "google.cloud.parallelstore", "ga"},
		{"Other version", "google.cloud.parallelstore.v2", "ga"},
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
			got, _ := getVerb(test.methodName)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetPluralFromSegments(t *testing.T) {
	for _, test := range []struct {
		name     string
		segments []api.PathSegment
		want     string
	}{
		{
			name:     "Standard",
			segments: []api.PathSegment{lit("projects"), variable("project"), lit("locations"), variable("location"), lit("instances"), variable("instance")},
			want:     "instances",
		},
		{
			name:     "Short",
			segments: []api.PathSegment{lit("shelves"), variable("shelf")},
			want:     "shelves",
		},
		{
			name:     "No Variable End",
			segments: []api.PathSegment{lit("projects"), variable("project"), lit("locations")},
			want:     "",
		},
		{
			name:     "Empty",
			segments: nil,
			want:     "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := getPluralFromSegments(test.segments)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetSingularFromSegments(t *testing.T) {
	for _, test := range []struct {
		name     string
		segments []api.PathSegment
		want     string
	}{
		{
			name:     "Standard",
			segments: []api.PathSegment{lit("projects"), variable("project"), lit("locations"), variable("location"), lit("instances"), variable("instance")},
			want:     "instance",
		},
		{
			name:     "Short",
			segments: []api.PathSegment{lit("shelves"), variable("shelf")},
			want:     "shelf",
		},
		{
			name:     "No Variable End",
			segments: []api.PathSegment{lit("projects"), variable("project"), lit("locations")},
			want:     "",
		},
		{
			name:     "Empty",
			segments: nil,
			want:     "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := getSingularFromSegments(test.segments)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetCollectionPathFromSegments(t *testing.T) {
	for _, test := range []struct {
		name     string
		segments []api.PathSegment
		want     string
	}{
		{
			name:     "Standard",
			segments: []api.PathSegment{lit("projects"), variable("project"), lit("locations"), variable("location"), lit("instances"), variable("instance")},
			want:     "projects.locations.instances",
		},
		{
			name:     "Short",
			segments: []api.PathSegment{lit("shelves"), variable("shelf")},
			want:     "shelves",
		},
		{
			name:     "Root",
			segments: []api.PathSegment{lit("projects"), variable("project")},
			want:     "projects",
		},
		{
			name:     "Mixed",
			segments: []api.PathSegment{lit("organizations"), variable("organization"), lit("locations"), variable("location"), lit("clusters"), variable("cluster")},
			want:     "organizations.locations.clusters",
		},
		{
			name:     "Global",
			segments: []api.PathSegment{lit("projects"), variable("project"), lit("global"), lit("networks"), variable("network")},
			want:     "projects.networks",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := getCollectionPathFromSegments(test.segments)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

