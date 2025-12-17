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
	"strings"

	"github.com/googleapis/librarian/internal/sidekick/api"
)

// GetPluralFromSegments infers the plural name of a resource from its structured path segments.
// Per AIP-122, the plural is the literal segment before the final variable segment.
// Example: `.../instances/{instance}` -> "instances"
func GetPluralFromSegments(segments []api.PathSegment) string {
	if len(segments) >= 2 {
		lastSegment := segments[len(segments)-1]
		if lastSegment.Variable != nil {
			// The second to last segment should be the literal plural name
			secondLastSegment := segments[len(segments)-2]
			if secondLastSegment.Literal != nil {
				return *secondLastSegment.Literal
			}
		}
	}
	return ""
}

// GetSingularFromSegments infers the singular name of a resource from its structured path segments.
// The singular is the name of the final variable segment.
// Example: `.../instances/{instance}` -> "instance"
func GetSingularFromSegments(segments []api.PathSegment) string {
	if len(segments) > 0 {
		last := segments[len(segments)-1]
		if last.Variable != nil && len(last.Variable.FieldPath) > 0 {
			// Typically the variable name is the last component of the field path
			// e.g. for `name` binding it might be implied? No, httprule parser populates FieldPath.
			return last.Variable.FieldPath[len(last.Variable.FieldPath)-1]
		}
	}
	return ""
}

// GetCollectionPathFromSegments constructs the base gcloud collection path from a
// structured resource pattern, according to AIP-122 conventions.
// It joins the literal collection identifiers with dots.
// Example: `projects/{project}/locations/{location}/instances/{instance}` -> `projects.locations.instances`
func GetCollectionPathFromSegments(segments []api.PathSegment) string {
	var collectionParts []string
	for i := 0; i < len(segments)-1; i++ {
		// A collection identifier is a literal segment followed by a variable segment.
		if segments[i].Literal != nil && segments[i+1].Variable != nil {
			collectionParts = append(collectionParts, *segments[i].Literal)
		}
	}
	return strings.Join(collectionParts, ".")
}
