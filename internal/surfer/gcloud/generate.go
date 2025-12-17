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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/googleapis/librarian/internal/sidekick/api"
	"github.com/googleapis/librarian/internal/sidekick/config"
	"github.com/googleapis/librarian/internal/sidekick/parser"
	"github.com/googleapis/librarian/internal/surfer/gcloud/utils"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v3"
)

// ==========================================
// Main Entrypoint
// ==========================================

// Generate generates gcloud commands for a service.
func Generate(ctx context.Context, googleapis, gcloudconfig, output, includeList string) error {
	overrides, err := readGcloudConfig(gcloudconfig)
	if err != nil {
		return err
	}

	model, err := createAPIModel(googleapis, includeList)
	if err != nil {
		return err
	}

	if len(model.Services) == 0 {
		return fmt.Errorf("no services found in the provided protos")
	}

	for _, service := range model.Services {
		// TODO(issues/support_multiple_services.md): Ensure output directories don't collide if multiple services share a name.
		if err := generateService(service, overrides, model, output); err != nil {
			return fmt.Errorf("failed to generate commands for service %q: %w", service.Name, err)
		}
	}
	return nil
}

func createAPIModel(googleapisPath, includeList string) (*api.API, error) {
	parserConfig := &config.Config{
		General: config.GeneralConfig{
			SpecificationFormat: "protobuf",
		},
		Source: map[string]string{
			"local-root":   googleapisPath,
			"include-list": includeList,
		},
	}

	// We use `parser.CreateModel` instead of calling the individual parsing and processing
	// functions directly because CreateModel is the designated entry point that ensures
	// the API model is not only parsed but also fully linked (cross-referenced), validated,
	// and processed with all necessary configuration overrides. This guarantees a complete
	// and consistent model for the generator without code duplication. It's worth noting that
	// we don't use all the functionality of post-processing of CreateModel, so depending
	// on our needs, if we don't find ourselves needing the additional post-processing
	// functionality, we could write our own simpler `CreateModel` function
	model, err := parser.CreateModel(parserConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create API model: %w", err)
	}
	return model, nil
}

// readGcloudConfig loads the gcloud configuration from a gcloud.yaml file.
func readGcloudConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read gcloud config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse gcloud config YAML: %w", err)
	}
	return &cfg, nil
}

// ==========================================
// Service Processing
// ==========================================

func generateService(service *api.Service, overrides *Config, model *api.API, output string) error {
	// Determine short service name for directory structure.
	// The `shortServiceName` is derived from `service.DefaultHost` (e.g., "parallelstore.googleapis.com" -> "parallelstore").
	// `service.DefaultHost`  matches the name field in the service config file
	// (e.g., `default_host` for parallelstore is derived from `parallelstore_v1.yaml` name field).
	shortServiceName := ""
	hostParts := strings.Split(service.DefaultHost, ".")
	if len(hostParts) > 0 {
		shortServiceName = hostParts[0]
	}

	if shortServiceName == "" {
		return fmt.Errorf("failed to determine short service name for service %q: default_host is empty", service.Name)
	}

	// The final output will be placed in a directory structure like:
	// `{outdir}/{shortServiceName}/`
	surfaceDir := filepath.Join(output, shortServiceName)

	// gcloud commands are resource-centric commands (e.g., `gcloud parallelstore instances create`),
	// so we first need to group all the API methods by the resource they operate on.
	// We'll create a map where the key is the resource's collection ID (e.g., "instances")
	// and the value is a list of methods that act on that resource.
	methodsByResource := make(map[string][]*api.Method)

	for _, method := range service.Methods {
		// For each method, we determine the plural name of the resource it operates on.
		// This plural name (e.g., "instances") will serve as our collection ID.
		// Example: For the `CreateInstance` method, this will return "instances".
		collectionID := getPluralResourceNameForMethod(method, model)

		// If a collection ID is found, we add the method to our map.
		if collectionID != "" {
			methodsByResource[collectionID] = append(methodsByResource[collectionID], method)
		}
	}

	// Now that we have grouped the methods by resource, we can generate the
	// command files for each resource.
	for collectionID, methods := range methodsByResource {
		// The `generateResourceCommands` function will handle the creation of the
		// directory structure and YAML files for this specific resource.
		err := generateResourceCommands(collectionID, methods, surfaceDir, overrides, model, service)
		if err != nil {
			return err
		}
	}
	return nil
}

// generateResourceCommands creates the directory structure and YAML files for a
// single resource's commands (e.g., create, delete, list).
//
// For a given collectionID like "instances", this function will create a directory
// `instances/` and populate it with `create.yaml`, `delete.yaml`, etc.
func generateResourceCommands(collectionID string, methods []*api.Method, baseDir string, overrides *Config, model *api.API, service *api.Service) error {
	// The main directory for the resource is named after its collection ID.
	// Example: `{baseDir}/instances`
	resourceDir := filepath.Join(baseDir, collectionID)

	// Gcloud commands are defined in a `_partials` directory. This allows
	// for sharing command definitions across different release tracks (GA, Beta, Alpha).
	partialsDir := filepath.Join(resourceDir, "_partials")
	if err := os.MkdirAll(partialsDir, 0755); err != nil {
		return fmt.Errorf("failed to create partials directory for %q: %w", collectionID, err)
	}

	// We iterate through each method associated with this resource.
	for _, method := range methods {
		// We map the API method name to a standard gcloud command verb.
		// Example: `CreateInstance` -> "create"
		verb, err := utils.GetVerb(method.Name)
		if err != nil {
			// Continue to the next method if we can't determine a verb,
			// logging the issue might be useful here in the future.
			continue
		}

		// We construct the complete command definition from the API method.
		// This involves generating all the arguments, help text, and request details.
		cmd, err := newCommand(method, overrides, model, service)
		if err != nil {
			return err
		}

		// in gcloud convention, the final YAML file must contain a list of commands,
		// even if there is only one.
		cmdList := []*Command{cmd}

		// We create the main command file (e.g., `create.yaml`). This file doesn't
		// contain the command definition itself, but rather a directive that tells
		// gcloud to look in the `_partials` directory.
		mainCmdPath := filepath.Join(resourceDir, fmt.Sprintf("%s.yaml", verb))
		if err := os.WriteFile(mainCmdPath, []byte("_PARTIALS_: true\n"), 0644); err != nil {
			return fmt.Errorf("failed to write main command file for %q: %w", method.Name, err)
		}

		// Generate a partial file for each release track.
		for _, track := range cmd.ReleaseTracks {
			trackName := strings.ToLower(track)
			partialFileName := fmt.Sprintf("_%s_%s.yaml", verb, trackName)
			partialCmdPath := filepath.Join(partialsDir, partialFileName)

			// We marshal the command definition struct into YAML format.
			b, err := yaml.Marshal(cmdList)
			if err != nil {
				return fmt.Errorf("failed to marshal partial command for %q: %w", method.Name, err)
			}

			// Finally, we write the generated YAML to the partial file.
			if err := os.WriteFile(partialCmdPath, b, 0644); err != nil {
				return fmt.Errorf("failed to write partial command file for %q: %w", method.Name, err)
			}
		}
	}
	return nil
}

// ==========================================
// Command Generation
// ==========================================

// newCommand constructs a single gcloud command definition from an API method.
// This function assembles all the necessary pieces: help text, arguments,
// request details, and async configuration.
func newCommand(method *api.Method, overrides *Config, model *api.API, service *api.Service) (*Command, error) {
	// We look up the help text and API definition for this specific method in the
	// `gcloud.yaml` configuration file.
	rule := findHelpTextRule(method, overrides)

	// We initialize the command with some default values.
	cmd := &Command{
		AutoGenerated: true,
	}

	if len(overrides.APIs) > 0 {
		cmd.Hidden = overrides.APIs[0].RootIsHidden
	} else {
		// Default to hidden if no API overrides are provided.
		cmd.Hidden = true
	}

	// If a help text rule was found in the config, we apply it to the command.
	if rule != nil {
		cmd.HelpText = HelpText{
			Brief:       rule.HelpText.Brief,
			Description: rule.HelpText.Description,
			Examples:    strings.Join(rule.HelpText.Examples, "\n\n"),
		}
	}

	// Infer default release track from proto package.
	// TODO(issue/allow_config_override_for_tracks.md): Allow gcloud config to overwrite the track for this command.
	inferredTrack := utils.InferTrackFromPackage(method.Service.Package)
	cmd.ReleaseTracks = []string{strings.ToUpper(inferredTrack)}

	// The core of the command generation happens here: we generate the arguments,
	// request details, and async configuration.
	args, err := newArguments(method, overrides, model, service)
	if err != nil {
		return nil, err
	}
	cmd.Arguments = args
	cmd.Request = newRequest(method, overrides, model)
	if method.OperationInfo != nil {
		cmd.Async = newAsync(method, overrides)
	}

	return cmd, nil
}

// newArguments generates the set of arguments for a command by parsing the
// fields of the method's request message.
func newArguments(method *api.Method, overrides *Config, model *api.API, service *api.Service) (Arguments, error) {
	args := Arguments{}
	if method.InputType == nil {
		return args, nil
	}

	// We iterate over each field in the method's request message (e.g., `CreateInstanceRequest`).
	for _, field := range method.InputType.Fields {
		// The "parent" field is a special case. Its information is captured by the
		// primary resource argument, so we skip it here to avoid creating a redundant flag.
		if field.Name == "parent" {
			continue
		}

		// We check if the current field represents the primary resource of the command.
		// For example, in a `CreateInstance` method, this would be the `instance_id` field.
		if isPrimaryResource(field, method) {
			// If it is the primary resource, we generate a special positional argument for it.
			param := newPrimaryResourceParam(field, method, model, overrides, service)
			args.Params = append(args.Params, param)
			continue
		}

		// For all other fields, we generate a standard flag argument. If the field
		// is a nested message, its fields will be "flattened" into top-level flags.
		// For example, a field `instance.description` becomes the `--description` flag.
		if err := addFlattenedParams(field, field.JSONName, &args, overrides, model, service); err != nil {
			return Arguments{}, err
		}
	}
	return args, nil
}

// addFlattenedParams recursively processes a field and its sub-fields to generate
// a flat list of command-line flags. This is necessary for nested messages in
// the request proto.
func addFlattenedParams(field *api.Field, prefix string, args *Arguments, overrides *Config, model *api.API, service *api.Service) error {
	// We skip fields that are marked as `OUTPUT_ONLY` in the proto, as these are
	// not meant to be provided by the user. We also skip the "name" field, as it's
	// handled by the primary resource argument.
	if slices.Contains(field.Behavior, api.FIELD_BEHAVIOR_OUTPUT_ONLY) || field.Name == "name" {
		return nil
	}

	// If the field is a nested message (and not a map, which is handled differently),
	// we need to recurse into its fields. This is the "flattening" process.
	// For example, in the Parallelstore API, the `CreateInstanceRequest` message
	// has a field named `instance` which is of type `Instance`. The `Instance`
	// message itself has fields like `description` and `capacity_gib`.
	// This block will recurse into the `Instance` message's fields.
	if field.MessageType != nil && !field.Map {
		for _, f := range field.MessageType.Fields {
			// The prefix is updated to create a dot-separated path for the `api_field`.
			// Continuing the example: when processing the `capacity_gib` field inside the
			// `Instance` message, the prefix will become "instance.capacityGib". This
			// results in a `--capacity-gib` flag that maps to the correct nested field.
			if err := addFlattenedParams(f, fmt.Sprintf("%s.%s", prefix, f.JSONName), args, overrides, model, service); err != nil {
				return err
			}
		}
		return nil
	}

	// If the field is a scalar, map, or enum, we generate a parameter for it.
	param, err := newParam(field, prefix, overrides, model, service)
	if err != nil {
		return err
	}
	args.Params = append(args.Params, param)
	return nil
}

// newParam creates a single command-line argument (a `Param` struct) from a proto field.
func newParam(field *api.Field, apiField string, overrides *Config, model *api.API, service *api.Service) (Param, error) {
	// We initialize the Param with the basic information derived from the field.
	param := Param{
		// The command-line flag name is the kebab-case version of the field name.
		// Example: `requestId` -> `request-id`
		ArgName: strcase.ToKebab(field.Name),
		// The `api_field` is the dot-separated path to the field in the request message.
		APIField: apiField,
		// We determine if the field is required based on the `(google.api.field_behavior)` annotation.
		Required: field.DocumentAsRequired(),
		// We check if the field is repeated in the proto.
		Repeated: field.Repeated,
	}

	// Now we handle the different types of fields.
	if field.ResourceReference != nil {
		// If the field is a resource reference (e.g., a field for a network), we
		// generate a `ResourceSpec` for it. This tells gcloud how to parse the
		// resource name provided by the user.
		spec, err := newResourceReferenceSpec(field, model, overrides, service)
		if err != nil {
			return Param{}, err
		}
		param.ResourceSpec = spec
		param.ResourceMethodParams = map[string]string{
			apiField: "{__relative_name__}",
		}
	} else if field.Map {
		// If the field is a map, we generate a spec for its key-value pairs.
		param.Repeated = true
		param.Spec = []ArgSpec{
			{APIField: "key"},
			{APIField: "value"},
		}
	} else if field.EnumType != nil {
		// If the field is an enum, we generate a list of choices for the flag.
		for _, v := range field.EnumType.Values {
			// We skip the default "UNSPECIFIED" value, as it's not a valid choice for the user.
			if strings.HasSuffix(v.Name, "_UNSPECIFIED") {
				continue
			}
			param.Choices = append(param.Choices, Choice{
				ArgValue:  strcase.ToKebab(v.Name),
				EnumValue: v.Name,
			})
		}
	} else {
		// If it's a scalar type (string, int, bool, etc.), we map its proto type
		// to the corresponding gcloud type.
		param.Type = utils.GetGcloudType(field.Typez)
	}

	// We try to find help text for this field in the `gcloud.yaml` config.
	// If none is found, we generate a default help text.
	if rule := findFieldHelpTextRule(field, overrides); rule != nil {
		param.HelpText = rule.HelpText.Brief
	} else {
		// TODO(https://github.com/googleapis/librarian/issues/3033): improve default help text inference
		param.HelpText = fmt.Sprintf("Value for the `%s` field.", strcase.ToKebab(field.Name))
	}
	return param, nil
}

// newPrimaryResourceParam creates the main positional resource argument for a command.
// This is the argument that represents the resource being acted upon (e.g., the instance name).
func newPrimaryResourceParam(field *api.Field, method *api.Method, model *api.API, _ *Config, service *api.Service) Param {
	// We first need to get the full resource definition for the method.
	resource := getResourceForMethod(method, model)
	var segments []api.PathSegment
	if resource != nil && len(resource.Patterns) > 0 {
		segments = resource.Patterns[0]
	}

	// We construct the gcloud collection path from the resource's pattern string.
	// Example: `projects/{project}/locations/{location}/instances/{instance}` -> `projects.locations.instances`
	collectionPath := utils.GetCollectionPathFromSegments(segments)
	hostParts := strings.Split(service.DefaultHost, ".")
	shortServiceName := hostParts[0]

	// We determine the singular name of the resource.
	// For `Create` methods, this comes from the `_id` field. For others, it's the `name` field.
	resourceName := strcase.ToSnake(strings.TrimSuffix(field.Name, "_id"))
	if field.Name == "name" {
		resourceName = utils.GetSingularFromSegments(segments)
	}

	// We generate a helpful help text based on whether the command is a `Create` command or not.
	helpText := fmt.Sprintf("The %s to create.", resourceName)
	if !strings.HasPrefix(method.Name, "Create") {
		helpText = fmt.Sprintf("The %s to operate on.", resourceName)
	}

	// We assemble the final `Param` struct with all the necessary information for a primary resource.
	return Param{
		HelpText:          helpText,
		IsPositional:      true,
		IsPrimaryResource: true,
		Required:          true,
		RequestIDField:    strcase.ToLowerCamel(field.Name),
		ResourceSpec: &ResourceSpec{
			Name:                  resourceName,
			PluralName:            getPluralResourceNameForMethod(method, model),
			Collection:            fmt.Sprintf("%s.%s", shortServiceName, collectionPath),
			DisableAutoCompleters: false,
			Attributes:            newAttributesFromSegments(segments),
		},
	}
}

// newResourceReferenceSpec creates a ResourceSpec for a field that references
// another resource type (e.g., a `--network` flag).
func newResourceReferenceSpec(field *api.Field, model *api.API, overrides *Config, service *api.Service) (*ResourceSpec, error) {
	// We iterate through all the resource definitions in the API model to find the
	// one that matches the type of our resource reference.
	for _, def := range model.ResourceDefinitions {
		if def.Type == field.ResourceReference.Type {
			if len(def.Patterns) == 0 {
				return nil, fmt.Errorf("resource definition for %q has no patterns", def.Type)
			}
			segments := def.Patterns[0]

			// We determine the plural name, using the explicit `plural` field if available,
			// and falling back to parsing the pattern otherwise.
			pluralName := def.Plural
			if pluralName == "" {
				pluralName = utils.GetPluralFromSegments(segments)
			}

			// We determine the singular name from the pattern.
			name := utils.GetSingularFromSegments(segments)

			// We construct the full gcloud collection path for the referenced resource
			//assuming the current service is the current command.
			hostParts := strings.Split(service.DefaultHost, ".")
			shortServiceName := hostParts[0]
			baseCollectionPath := utils.GetCollectionPathFromSegments(segments)
			fullCollectionPath := fmt.Sprintf("%s.%s", shortServiceName, baseCollectionPath)

			// We assemble and return the `ResourceSpec`.
			return &ResourceSpec{
				Name:                  name,
				PluralName:            pluralName,
				Collection:            fullCollectionPath,
				DisableAutoCompleters: true,
				Attributes:            newAttributesFromSegments(segments),
			}, nil
		}
	}
	return nil, fmt.Errorf("resource definition not found for type %q", field.ResourceReference.Type)
}

// newAttributesFromPattern parses a resource pattern string (e.g.,
// "projects/{project}/locations/{location}") and extracts the attributes
// that make up the resource's name.
func newAttributesFromPattern(pattern string) []Attribute {
	var attributes []Attribute
	parts := strings.Split(pattern, "/")

	// We iterate over the segments of the pattern.
	for i, part := range parts {
		// A variable segment is enclosed in curly braces.
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			// The `attribute_name` is the name of the variable (e.g., "project").
			name := strings.Trim(part, "{}")
			var parameterName string

			// The `parameter_name` is derived from the preceding literal segment
			// (e.g., "projects" -> "projectsId"). This is a gcloud convention.
			if i > 0 {
				parameterName = parts[i-1] + "Id"
			} else {
				// This is a fallback for the unlikely case that a pattern starts with a variable.
				parameterName = name + "sId"
			}

			attr := Attribute{
				AttributeName: name,
				ParameterName: parameterName,
				Help:          fmt.Sprintf("The %s id of the {resource} resource.", name),
			}

			// If the attribute is a project, we add the standard gcloud property fallback,
			// so users don't have to specify `--project` if it's already configured.
			if name == "project" {
				attr.Property = "core/project"
			}
			attributes = append(attributes, attr)
		}
	}
	return attributes
}

// newRequest creates the `Request` part of the command definition.
func newRequest(method *api.Method, overrides *Config, model *api.API) *Request {
	// TODO(issues/dynamic_request_async_collection.md): The collection path is partially hardcoded.
	return &Request{
		APIVersion: apiVersion(overrides),
		Collection: []string{fmt.Sprintf("parallelstore.projects.locations.%s", getPluralResourceNameForMethod(method, model))},
	}
}

// newAsync creates the `Async` part of the command definition for long-running operations.
func newAsync(method *api.Method, overrides *Config) *Async {
	return &Async{
		// TODO(issues/dynamic_request_async_collection.md): The collection path is partially hardcoded.
		Collection: []string{"parallelstore.projects.locations.operations"},
	}
}

// ==========================================
// Resource Helpers
// ==========================================

// isPrimaryResource determines if a field represents the primary resource of a method.
func isPrimaryResource(field *api.Field, method *api.Method) bool {
	if method.InputType == nil {
		return false
	}
	// For `Create` methods, the primary resource is identified by a field named
	// in the format "{resource}_id" (e.g., "instance_id").
	if strings.HasPrefix(method.Name, "Create") {
		resourceName := getResourceName(method)
		if resourceName != "" && field.Name == strcase.ToSnake(resourceName)+"_id" {
			return true
		}
	}
	// For `Get`, `Delete`, and `Update` methods, the primary resource is identified
	// by a field named "name", which holds the full resource name.
	if (strings.HasPrefix(method.Name, "Get") || strings.HasPrefix(method.Name, "Delete") || strings.HasPrefix(method.Name, "Update")) && field.Name == "name" {
		return true
	}
	return false
}

// getResourceName extracts the name of the resource from a method's input message.
// For example, for `CreateInstanceRequest`, it would return "Instance".
func getResourceName(method *api.Method) string {
	for _, f := range method.InputType.Fields {
		if msg := f.MessageType; msg != nil && msg.Resource != nil {
			return msg.Name
		}
	}
	return ""
}

// getResourceForMethod finds the `api.Resource` definition associated with a method.
// This is a crucial function for linking a method to the resource it operates on.
func getResourceForMethod(method *api.Method, model *api.API) *api.Resource {
	if method.InputType == nil {
		return nil
	}

	// Strategy 1: For `Create` and `Update`, the request message usually contains
	// a field that *is* the resource message. This message is annotated with `(google.api.resource)`.
	for _, f := range method.InputType.Fields {
		if msg := f.MessageType; msg != nil && msg.Resource != nil {
			return msg.Resource
		}
	}

	// Strategy 2: For `Get`, `Delete`, and `List`, the request message has a `name`
	// or `parent` field with a `(google.api.resource_reference)`.
	var resourceType string
	for _, field := range method.InputType.Fields {
		if (field.Name == "name" || field.Name == "parent") && field.ResourceReference != nil {
			// For collection methods (like List), the reference is to the parent,
			// and the resource we care about is the `child_type`.
			if field.ResourceReference.ChildType != "" {
				resourceType = field.ResourceReference.ChildType
			} else {
				resourceType = field.ResourceReference.Type
			}
			break
		}
	}

	if resourceType == "" {
		return nil
	}

	// Use the API model's indexed maps for an efficient lookup.
	for _, r := range model.ResourceDefinitions {
		if r.Type == resourceType {
			return r
		}
	}

	// Also check resources defined on messages directly.
	for _, m := range model.Messages {
		if m.Resource != nil && m.Resource.Type == resourceType {
			return m.Resource
		}
	}

	return nil
}

// getPluralResourceNameForMethod determines the plural name of a resource. It follows a clear
// hierarchy of truth: first, the explicit `plural` field in the resource
// definition, and second, inference from the resource pattern.
func getPluralResourceNameForMethod(method *api.Method, model *api.API) string {
	resource := getResourceForMethod(method, model)
	if resource != nil {
		// The `plural` field in the `(google.api.resource)` annotation is the
		// most authoritative source.
		if resource.Plural != "" {
			return resource.Plural
		}
		// If the `plural` field is not present, we fall back to inferring the
		// plural name from the resource's pattern string, as per AIP-122.
		if len(resource.Patterns) > 0 {
			return utils.GetPluralFromSegments(resource.Patterns[0])
		}
	}
	return ""
}

// findHelpTextRule finds the help text rule from the config that applies to the current method.
func findHelpTextRule(method *api.Method, overrides *Config) *HelpTextRule {
	if overrides.APIs == nil {
		return nil
	}
	for _, api := range overrides.APIs {
		if api.HelpText == nil {
			continue
		}
		for _, rule := range api.HelpText.MethodRules {
			if rule.Selector == strings.TrimPrefix(method.ID, ".") {
				return rule
			}
		}
	}
	return nil
}

// findFieldHelpTextRule finds the help text rule from the config that applies to the current field.
func findFieldHelpTextRule(field *api.Field, overrides *Config) *HelpTextRule {
	if overrides.APIs == nil {
		return nil
	}
	for _, api := range overrides.APIs {
		if api.HelpText == nil {
			continue
		}
		for _, rule := range api.HelpText.FieldRules {
			if rule.Selector == field.ID {
				return rule
			}
		}
	}
	return nil
}

// apiVersion extracts the API version from the configuration.
func apiVersion(overrides *Config) string {
	if len(overrides.APIs) > 0 {
		return overrides.APIs[0].APIVersion
	}
	return ""
}

// newAttributesFromSegments parses a structured resource pattern and extracts the attributes
// that make up the resource's name.
func newAttributesFromSegments(segments []api.PathSegment) []Attribute {
	var attributes []Attribute

	// We iterate over the segments of the pattern.
	for i, part := range segments {
		// A variable segment is enclosed in curly braces.
		if part.Variable != nil {
			// The `attribute_name` is the name of the variable (e.g., "project").
			if len(part.Variable.FieldPath) == 0 {
				continue
			}
			name := part.Variable.FieldPath[len(part.Variable.FieldPath)-1]
			var parameterName string

			// The `parameter_name` is derived from the preceding literal segment
			// (e.g., "projects" -> "projectsId"). This is a gcloud convention.
			if i > 0 && segments[i-1].Literal != nil {
				parameterName = *segments[i-1].Literal + "Id"
			} else {
				// This is a fallback for the unlikely case that a pattern starts with a variable.
				parameterName = name + "sId"
			}

			attr := Attribute{
				AttributeName: name,
				ParameterName: parameterName,
				Help:          fmt.Sprintf("The %s id of the {resource} resource.", name),
			}

			// If the attribute is a project, we add the standard gcloud property fallback,
			// so users don't have to specify `--project` if it's already configured.
			if name == "project" {
				attr.Property = "core/project"
			}
			attributes = append(attributes, attr)
		}
	}
	return attributes
}
