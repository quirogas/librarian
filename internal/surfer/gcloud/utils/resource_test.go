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

package utils

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

func TestGetPluralFromSegments(t *testing.T) {
	for _, test := range []struct {
		name     string
		segments []api.PathSegment
		want     string
	}{
		{
			name: "Standard",
			segments: []api.PathSegment{
				*api.NewPathSegment().WithLiteral("projects"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("project").WithMatch()),
				*api.NewPathSegment().WithLiteral("locations"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("location").WithMatch()),
				*api.NewPathSegment().WithLiteral("instances"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("instance").WithMatch()),
			},
			want: "instances",
		},
		{
			name: "Short",
			segments: []api.PathSegment{
				*api.NewPathSegment().WithLiteral("shelves"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("shelf").WithMatch()),
			},
			want: "shelves",
		},
		{
			name: "No Variable End",
			segments: []api.PathSegment{
				*api.NewPathSegment().WithLiteral("projects"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("project").WithMatch()),
				*api.NewPathSegment().WithLiteral("locations"),
			},
			want: "",
		},
		{
			name:     "Empty",
			segments: nil,
			want:     "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := GetPluralFromSegments(test.segments)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("GetPluralFromSegments mismatch (-want +got):\n%s", diff)
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
			name: "Standard",
			segments: []api.PathSegment{
				*api.NewPathSegment().WithLiteral("projects"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("project").WithMatch()),
				*api.NewPathSegment().WithLiteral("locations"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("location").WithMatch()),
				*api.NewPathSegment().WithLiteral("instances"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("instance").WithMatch()),
			},
			want: "instance",
		},
		{
			name: "Short",
			segments: []api.PathSegment{
				*api.NewPathSegment().WithLiteral("shelves"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("shelf").WithMatch()),
			},
			want: "shelf",
		},
		{
			name: "No Variable End",
			segments: []api.PathSegment{
				*api.NewPathSegment().WithLiteral("projects"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("project").WithMatch()),
				*api.NewPathSegment().WithLiteral("locations"),
			},
			want: "",
		},
		{
			name:     "Empty",
			segments: nil,
			want:     "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := GetSingularFromSegments(test.segments)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("GetSingularFromSegments mismatch (-want +got):\n%s", diff)
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
			name: "Standard",
			segments: []api.PathSegment{
				*api.NewPathSegment().WithLiteral("projects"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("project").WithMatch()),
				*api.NewPathSegment().WithLiteral("locations"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("location").WithMatch()),
				*api.NewPathSegment().WithLiteral("instances"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("instance").WithMatch()),
			},
			want: "projects.locations.instances",
		},
		{
			name: "Short",
			segments: []api.PathSegment{
				*api.NewPathSegment().WithLiteral("shelves"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("shelf").WithMatch()),
			},
			want: "shelves",
		},
		{
			name: "Root",
			segments: []api.PathSegment{
				*api.NewPathSegment().WithLiteral("projects"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("project").WithMatch()),
			},
			want: "projects",
		},
		{
			name: "Mixed",
			segments: []api.PathSegment{
				*api.NewPathSegment().WithLiteral("organizations"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("organization").WithMatch()),
				*api.NewPathSegment().WithLiteral("locations"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("location").WithMatch()),
				*api.NewPathSegment().WithLiteral("clusters"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("cluster").WithMatch()),
			},
			want: "organizations.locations.clusters",
		},
		{
			name: "Global",
			segments: []api.PathSegment{
				*api.NewPathSegment().WithLiteral("projects"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("project").WithMatch()),
				*api.NewPathSegment().WithLiteral("global"),
				*api.NewPathSegment().WithLiteral("networks"),
				*api.NewPathSegment().WithVariable(api.NewPathVariable("network").WithMatch()),
			},
			want: "projects.networks",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := GetCollectionPathFromSegments(test.segments)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("GetCollectionPathFromSegments mismatch (-want +got):\n%s", diff)
			}
		})
	}
}