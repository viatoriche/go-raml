package raml

import (
	"fmt"
	"regexp"
	"strconv"

	orderedmap "github.com/wk8/go-ordered-map/v2"
	"gopkg.in/yaml.v3"

	"github.com/acronis/go-stacktrace"
)

// ArrayFacets contains constraints for array shapes.
type ArrayFacets struct {
	Items       *BaseShape
	MinItems    *uint64
	MaxItems    *uint64
	UniqueItems *bool
}

// ArrayShape represents an array shape.
type ArrayShape struct {
	*BaseShape

	ArrayFacets
}

// Base returns the base shape.
func (s *ArrayShape) Base() *BaseShape {
	return s.BaseShape
}

func (s *ArrayShape) clone(base *BaseShape, clonedMap map[int64]*BaseShape) Shape {
	c := *s
	c.BaseShape = base
	if c.Items != nil {
		c.Items = c.Items.clone(clonedMap)
	}
	return &c
}

func (s *ArrayShape) validate(v interface{}, ctxPath string) error {
	i, ok := v.([]interface{})
	if !ok {
		return fmt.Errorf("invalid type, got %T, expected []interface{}", v)
	}

	arrayLen := uint64(len(i))
	if s.MinItems != nil && arrayLen < *s.MinItems {
		return fmt.Errorf("array must have at least %d items", *s.MinItems)
	}
	if s.MaxItems != nil && arrayLen > *s.MaxItems {
		return fmt.Errorf("array must have not more than %d items", *s.MaxItems)
	}
	validateUniqueItems := s.UniqueItems != nil && *s.UniqueItems
	uniqueItems := make(map[interface{}]struct{})
	for ii, item := range i {
		ctxPathA := ctxPath + "[" + strconv.Itoa(ii) + "]"
		if s.Items != nil {
			if err := s.Items.Shape.validate(item, ctxPathA); err != nil {
				return fmt.Errorf("validate array item %s: %w", ctxPathA, err)
			}
		}
		if validateUniqueItems {
			uniqueItems[item] = struct{}{}
		}
	}
	if validateUniqueItems && len(uniqueItems) != len(i) {
		return fmt.Errorf("array contains duplicate items")
	}

	return nil
}

// Inherit merges the source shape into the target shape.
func (s *ArrayShape) inherit(source Shape) (Shape, error) {
	ss, ok := source.(*ArrayShape)
	if !ok {
		return nil, stacktrace.New("cannot inherit from different type", s.Location,
			stacktrace.WithPosition(&s.Position), stacktrace.WithInfo("source", source.Base().Type),
			stacktrace.WithInfo("target", s.Type))
	}
	if s.Items == nil {
		s.Items = ss.Items
	} else if ss.Items != nil {
		_, err := s.Items.Inherit(ss.Items)
		if err != nil {
			return nil, StacktraceNewWrapped("merge array items", err, s.Location,
				stacktrace.WithPosition(&s.Items.Position))
		}
	}
	if s.MinItems == nil {
		s.MinItems = ss.MinItems
	} else if ss.MinItems != nil && *s.MinItems > *ss.MinItems {
		return nil, stacktrace.New("minItems constraint violation", s.Location,
			stacktrace.WithPosition(&s.Position), stacktrace.WithInfo("source", *ss.MinItems),
			stacktrace.WithInfo("target", *s.MinItems))
	}
	if s.MaxItems == nil {
		s.MaxItems = ss.MaxItems
	} else if ss.MaxItems != nil && *s.MaxItems < *ss.MaxItems {
		return nil, stacktrace.New("maxItems constraint violation", s.Location,
			stacktrace.WithPosition(&s.Position), stacktrace.WithInfo("source", *ss.MaxItems),
			stacktrace.WithInfo("target", *s.MaxItems))
	}
	if s.UniqueItems == nil {
		s.UniqueItems = ss.UniqueItems
	} else if ss.UniqueItems != nil && *ss.UniqueItems && !*s.UniqueItems {
		return nil, stacktrace.New("uniqueItems constraint violation", s.Location,
			stacktrace.WithPosition(&s.Position), stacktrace.WithInfo("source", *ss.UniqueItems),
			stacktrace.WithInfo("target", *s.UniqueItems))
	}
	return s, nil
}

func (s *ArrayShape) check() error {
	if s.MinItems != nil && s.MaxItems != nil && *s.MinItems > *s.MaxItems {
		return stacktrace.New("minItems must be less than or equal to maxItems", s.Location,
			stacktrace.WithPosition(&s.Position))
	}
	if s.Items != nil {
		if err := s.Items.Check(); err != nil {
			return StacktraceNewWrapped("check items", err, s.Location,
				stacktrace.WithPosition(&s.Items.Position))
		}
	}
	return nil
}

// UnmarshalYAMLNodes unmarshals the array shape from YAML nodes.
func (s *ArrayShape) unmarshalYAMLNodes(v []*yaml.Node) error {
	if len(v)%2 != 0 {
		return stacktrace.New("odd number of nodes", s.Location, stacktrace.WithPosition(&s.Position))
	}
	for i := 0; i != len(v); i += 2 {
		node := v[i]
		valueNode := v[i+1]

		switch node.Value {
		case FacetMinItems:
			if err := valueNode.Decode(&s.MinItems); err != nil {
				return StacktraceNewWrapped("decode", err, s.Location,
					WithNodePosition(valueNode),
					stacktrace.WithInfo("facet", FacetMinItems))
			}
		case FacetMaxItems:
			if err := valueNode.Decode(&s.MaxItems); err != nil {
				return StacktraceNewWrapped("decode", err, s.Location,
					WithNodePosition(valueNode),
					stacktrace.WithInfo("facet", FacetMaxItems))
			}
		case FacetItems:
			shape, err := s.raml.makeNewShapeYAML(valueNode, FacetItems, s.Location)
			if err != nil {
				return StacktraceNewWrapped("make shape", err, s.Location,
					WithNodePosition(valueNode),
					stacktrace.WithInfo("facet", FacetItems))
			}
			s.Items = shape
		case FacetUniqueItems:
			if err := valueNode.Decode(&s.UniqueItems); err != nil {
				return StacktraceNewWrapped("decode", err, s.Location,
					WithNodePosition(valueNode),
					stacktrace.WithInfo("facet", FacetUniqueItems))
			}
		default:
			n, err := s.raml.makeRootNode(valueNode, s.Location)
			if err != nil {
				return StacktraceNewWrapped("make node", err, s.Location, WithNodePosition(valueNode))
			}
			s.CustomShapeFacets.Set(node.Value, n)
		}
	}
	return nil
}

// ObjectFacets contains constraints for object shapes.
type ObjectFacets struct {
	Discriminator        *string
	DiscriminatorValue   any
	AdditionalProperties *bool
	Properties           *orderedmap.OrderedMap[string, Property]
	PatternProperties    *orderedmap.OrderedMap[string, PatternProperty]
	MinProperties        *uint64
	MaxProperties        *uint64
}

// ObjectShape represents an object shape.
type ObjectShape struct {
	*BaseShape

	ObjectFacets
}

func (s *ObjectShape) unmarshalPatternProperties(
	nodeName, propertyName string, data *yaml.Node, hasImplicitOptional bool) error {
	if s.PatternProperties == nil {
		s.PatternProperties = orderedmap.New[string, PatternProperty]()
	}
	property, err := s.raml.makePatternProperty(nodeName, propertyName, data, s.Location,
		hasImplicitOptional)
	if err != nil {
		return StacktraceNewWrapped("make pattern property", err, s.Location,
			WithNodePosition(data))
	}
	s.PatternProperties.Set(propertyName, property)
	return nil
}

func (s *ObjectShape) unmarshalProperty(nodeName string, data *yaml.Node) error {
	propertyName, hasImplicitOptional := s.raml.chompImplicitOptional(nodeName)
	if len(propertyName) > 1 && propertyName[0] == '/' && propertyName[len(propertyName)-1] == '/' {
		return s.unmarshalPatternProperties(nodeName, propertyName, data, hasImplicitOptional)
	}

	if s.Properties == nil {
		s.Properties = orderedmap.New[string, Property]()
	}
	property, err := s.raml.makeProperty(nodeName, propertyName, data, s.Location, hasImplicitOptional)
	if err != nil {
		return StacktraceNewWrapped("make property", err, s.Location, WithNodePosition(data))
	}
	s.Properties.Set(property.Name, property)
	return nil
}

// UnmarshalYAMLNodes unmarshals the object shape from YAML nodes.
func (s *ObjectShape) unmarshalYAMLNodes(v []*yaml.Node) error {
	for i := 0; i != len(v); i += 2 {
		node := v[i]
		valueNode := v[i+1]

		switch node.Value {
		case FacetAdditionalProperties:
			if err := valueNode.Decode(&s.AdditionalProperties); err != nil {
				return StacktraceNewWrapped("decode", err, s.Location,
					WithNodePosition(valueNode),
					stacktrace.WithInfo("facet", FacetAdditionalProperties))
			}
		case FacetDiscriminator:
			if err := valueNode.Decode(&s.Discriminator); err != nil {
				return StacktraceNewWrapped("decode", err, s.Location,
					WithNodePosition(valueNode),
					stacktrace.WithInfo("facet", FacetDiscriminator))
			}
		case FacetDiscriminatorValue:
			if err := valueNode.Decode(&s.DiscriminatorValue); err != nil {
				return StacktraceNewWrapped("decode", err, s.Location,
					WithNodePosition(valueNode),
					stacktrace.WithInfo("facet", FacetDiscriminatorValue))
			}
		case FacetMinProperties:
			if err := valueNode.Decode(&s.MinProperties); err != nil {
				return StacktraceNewWrapped("decode", err, s.Location,
					WithNodePosition(valueNode),
					stacktrace.WithInfo("facet", FacetMinProperties))
			}
		case FacetMaxProperties:
			if err := valueNode.Decode(&s.MaxProperties); err != nil {
				return StacktraceNewWrapped("decode", err, s.Location,
					WithNodePosition(valueNode),
					stacktrace.WithInfo("facet", FacetMaxProperties))
			}
		case FacetProperties:
			for j := 0; j != len(valueNode.Content); j += 2 {
				nodeName := valueNode.Content[j].Value
				data := valueNode.Content[j+1]

				if err := s.unmarshalProperty(nodeName, data); err != nil {
					return fmt.Errorf("unmarshal property: %w", err)
				}
			}
		default:
			n, err := s.raml.makeRootNode(valueNode, s.Location)
			if err != nil {
				return StacktraceNewWrapped("make node", err, s.Location, WithNodePosition(valueNode))
			}
			s.CustomShapeFacets.Set(node.Value, n)
		}
	}
	return nil
}

// Base returns the base shape.
func (s *ObjectShape) Base() *BaseShape {
	return s.BaseShape
}

func (s *ObjectShape) clone(base *BaseShape, clonedMap map[int64]*BaseShape) Shape {
	c := *s
	c.BaseShape = base
	if c.Properties != nil {
		c.Properties = orderedmap.New[string, Property](s.Properties.Len())
		for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
			k, prop := pair.Key, pair.Value
			prop.Shape = prop.Shape.clone(clonedMap)
			c.Properties.Set(k, prop)
		}
	}
	if c.PatternProperties != nil {
		c.PatternProperties = orderedmap.New[string, PatternProperty](s.PatternProperties.Len())
		for pair := s.PatternProperties.Oldest(); pair != nil; pair = pair.Next() {
			k, prop := pair.Key, pair.Value
			prop.Shape = prop.Shape.clone(clonedMap)
			c.PatternProperties.Set(k, prop)
		}
	}
	return &c
}

func (s *ObjectShape) validateProperties(ctxPath string, props map[string]interface{}) error {
	restrictedAdditionalProperties := s.AdditionalProperties != nil && !*s.AdditionalProperties
	for k, item := range props {
		// Explicitly defined properties have priority over pattern properties.
		ctxPathK := ctxPath + "." + k
		if s.Properties != nil {
			if p, present := s.Properties.Get(k); present {
				if err := p.Shape.Shape.validate(item, ctxPathK); err != nil {
					return fmt.Errorf("validate property %s: %w", ctxPathK, err)
				}
				continue
			}
		}
		if s.PatternProperties != nil {
			found := false
			for pair := s.PatternProperties.Oldest(); pair != nil; pair = pair.Next() {
				pp := pair.Value
				// NOTE: We validate only those keys that match the pattern.
				// The keys that do not match are considered as additional properties and are not validated.
				if pp.Pattern.MatchString(k) {
					// NOTE: The first defined pattern property to validate prevails.
					if err := pp.Shape.Shape.validate(item, ctxPathK); err == nil {
						found = true
						break
					}
				}
			}
			if found {
				continue
			}
		}
		// Will never happen if pattern properties are present.
		if restrictedAdditionalProperties {
			return fmt.Errorf("unexpected additional property \"%s\"", k)
		}
	}
	return nil
}

func (s *ObjectShape) validate(v interface{}, ctxPath string) error {
	props, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid type, got %T, expected map[string]interface{}", v)
	}

	if err := s.validateProperties(ctxPath, props); err != nil {
		return fmt.Errorf("validate properties: %w", err)
	}

	mapLen := uint64(len(props))
	if s.MinProperties != nil && mapLen < *s.MinProperties {
		return fmt.Errorf("object must have at least %d properties", *s.MinProperties)
	}
	if s.MaxProperties != nil && mapLen > *s.MaxProperties {
		return fmt.Errorf("object must have not more than %d properties", *s.MaxProperties)
	}

	return nil
}

func (s *ObjectShape) inheritMinProperties(source *ObjectShape) error {
	if s.MinProperties == nil {
		s.MinProperties = source.MinProperties
	} else if source.MinProperties != nil && *s.MinProperties > *source.MinProperties {
		return stacktrace.New("minProperties constraint violation", s.Location,
			stacktrace.WithPosition(&s.Position),
			stacktrace.WithInfo("source", *source.MinProperties),
			stacktrace.WithInfo("target", *s.MinProperties))
	}
	return nil
}

func (s *ObjectShape) inheritMaxProperties(source *ObjectShape) error {
	if s.MaxProperties == nil {
		s.MaxProperties = source.MaxProperties
	} else if source.MaxProperties != nil && *s.MaxProperties < *source.MaxProperties {
		return stacktrace.New("maxProperties constraint violation", s.Location,
			stacktrace.WithPosition(&s.Position),
			stacktrace.WithInfo("source", *source.MaxProperties),
			stacktrace.WithInfo("target", *s.MaxProperties))
	}
	return nil
}

func (s *ObjectShape) inheritProperties(source *ObjectShape) error {
	if s.Properties == nil {
		s.Properties = source.Properties
		return nil
	}

	if source.Properties == nil {
		return nil
	}

	for pair := source.Properties.Oldest(); pair != nil; pair = pair.Next() {
		k, sourceProp := pair.Key, pair.Value
		if targetProp, present := s.Properties.Get(k); present {
			if sourceProp.Required && !targetProp.Required {
				return stacktrace.New("cannot make required property optional", s.Location,
					stacktrace.WithPosition(&targetProp.Shape.Position),
					stacktrace.WithInfo("property", k),
					stacktrace.WithInfo("source", sourceProp.Required),
					stacktrace.WithInfo("target", targetProp.Required),
					stacktrace.WithType(stacktrace.TypeUnwrapping))
			}
			_, err := targetProp.Shape.Inherit(sourceProp.Shape)
			if err != nil {
				return StacktraceNewWrapped("inherit property", err, s.Location,
					stacktrace.WithPosition(&targetProp.Shape.Position),
					stacktrace.WithInfo("property", k),
					stacktrace.WithType(stacktrace.TypeUnwrapping))
			}
		} else {
			s.Properties.Set(k, sourceProp)
		}
	}
	return nil
}

func (s *ObjectShape) inheritPatternProperties(source *ObjectShape) error {
	if s.PatternProperties == nil {
		s.PatternProperties = source.PatternProperties
		return nil
	}
	if source.PatternProperties != nil {
		for pair := source.PatternProperties.Oldest(); pair != nil; pair = pair.Next() {
			k, sourceProp := pair.Key, pair.Value
			if targetProp, present := s.PatternProperties.Get(k); present {
				_, err := targetProp.Shape.Inherit(sourceProp.Shape)
				if err != nil {
					return StacktraceNewWrapped("inherit pattern property", err, s.Location,
						stacktrace.WithPosition(&targetProp.Shape.Position),
						stacktrace.WithInfo("property", k),
						stacktrace.WithType(stacktrace.TypeUnwrapping))
				}
			} else {
				s.PatternProperties.Set(k, sourceProp)
			}
		}
	}
	return nil
}

// Inherit merges the source shape into the target shape.
func (s *ObjectShape) inherit(source Shape) (Shape, error) {
	if ss, ok := source.(*RecursiveShape); ok {
		source = ss.Head.Shape
	}
	ss, ok := source.(*ObjectShape)
	if !ok {
		return nil, stacktrace.New("cannot inherit from different type", s.Location,
			stacktrace.WithPosition(&s.Position),
			stacktrace.WithInfo("source", source.Base().Type),
			stacktrace.WithInfo("target", s.Base().Type))
	}

	// Discriminator and AdditionalProperties are inherited as is
	if s.AdditionalProperties == nil {
		s.AdditionalProperties = ss.AdditionalProperties
	}
	if s.Discriminator == nil {
		s.Discriminator = ss.Discriminator
	}

	if err := s.inheritMinProperties(ss); err != nil {
		return nil, fmt.Errorf("inherit minProperties: %w", err)
	}

	if err := s.inheritMaxProperties(ss); err != nil {
		return nil, fmt.Errorf("inherit maxProperties: %w", err)
	}

	if err := s.inheritProperties(ss); err != nil {
		return nil, fmt.Errorf("inherit properties: %w", err)
	}

	if err := s.inheritPatternProperties(ss); err != nil {
		return nil, fmt.Errorf("inherit pattern properties: %w", err)
	}

	return s, nil
}

func (s *ObjectShape) checkPatternProperties() error {
	if s.PatternProperties == nil {
		return nil
	}
	if s.AdditionalProperties != nil && !*s.AdditionalProperties {
		// TODO: We actually can allow pattern properties with "additionalProperties: false" for stricter
		// 	validation.
		// This will contradict RAML 1.0 spec, but JSON Schema allows that.
		// https://json-schema.org/understanding-json-schema/reference/object#additionalproperties
		return stacktrace.New("pattern properties are not allowed with \"additionalProperties: false\"",
			s.Location, stacktrace.WithPosition(&s.Position))
	}
	for pair := s.PatternProperties.Oldest(); pair != nil; pair = pair.Next() {
		prop := pair.Value
		if err := prop.Shape.Check(); err != nil {
			return StacktraceNewWrapped("check pattern property", err, s.Location,
				stacktrace.WithPosition(&prop.Shape.Position),
				stacktrace.WithInfo("property", prop.Pattern.String()))
		}
	}
	return nil
}

func (s *ObjectShape) checkProperties() error {
	if s.Properties == nil {
		return nil
	}

	for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
		prop := pair.Value
		if err := prop.Shape.Check(); err != nil {
			return StacktraceNewWrapped("check property", err, s.Location,
				stacktrace.WithPosition(&prop.Shape.Position),
				stacktrace.WithInfo("property", prop.Name))
		}
	}
	// FIXME: Need to validate on which level the discriminator is applied to avoid potential false positives.
	// Inline definitions with discriminator are not allowed.
	if s.Discriminator != nil {
		prop, ok := s.Properties.Get(*s.Discriminator)
		if !ok {
			return stacktrace.New("discriminator property not found", s.Location,
				stacktrace.WithPosition(&s.Position),
				stacktrace.WithInfo("discriminator", *s.Discriminator))
		}
		if prop.Shape.IsScalar() {
			return stacktrace.New("discriminator property must be a scalar", s.Location,
				stacktrace.WithPosition(&prop.Shape.Position),
				stacktrace.WithInfo("discriminator", *s.Discriminator))
		}
		discriminatorValue := s.DiscriminatorValue
		if discriminatorValue == nil {
			discriminatorValue = s.Base().Name
		}
		if err := prop.Shape.Validate(discriminatorValue); err != nil {
			return StacktraceNewWrapped("validate discriminator value", err, s.Location,
				stacktrace.WithPosition(&s.Base().Position),
				stacktrace.WithInfo("discriminator", *s.Discriminator))
		}
	}

	return nil
}

func (s *ObjectShape) check() error {
	if s.MinProperties != nil && s.MaxProperties != nil && *s.MinProperties > *s.MaxProperties {
		return stacktrace.New("minProperties must be less than or equal to maxProperties",
			s.Location, stacktrace.WithPosition(&s.Position))
	}
	if err := s.checkPatternProperties(); err != nil {
		return fmt.Errorf("check pattern properties: %w", err)
	}
	if err := s.checkProperties(); err != nil {
		return fmt.Errorf("check properties: %w", err)
	}
	if s.Discriminator != nil && s.Properties == nil {
		return stacktrace.New("discriminator without properties", s.Location,
			stacktrace.WithPosition(&s.Position))
	}
	return nil
}

// makeProperty creates a pattern property from a YAML node.
func (r *RAML) makePatternProperty(nodeName string, propertyName string, v *yaml.Node, location string,
	hasImplicitOptional bool) (PatternProperty, error) {
	shape, err := r.makeNewShapeYAML(v, nodeName, location)
	if err != nil {
		return PatternProperty{}, StacktraceNewWrapped("make shape", err, location,
			WithNodePosition(v))
	}
	// Pattern properties cannot be required
	if shape.Required != nil || hasImplicitOptional {
		return PatternProperty{}, stacktrace.New("'required' facet is not supported on pattern property",
			location, WithNodePosition(v))
	}
	re, err := regexp.Compile(propertyName[1 : len(propertyName)-1])
	if err != nil {
		return PatternProperty{}, StacktraceNewWrapped("compile pattern", err, location, WithNodePosition(v))
	}
	return PatternProperty{
		Pattern: re,
		Shape:   shape,
		raml:    r,
	}, nil
}

func (r *RAML) chompImplicitOptional(nodeName string) (string, bool) {
	nameLen := len(nodeName)
	if nodeName != "" && nodeName[nameLen-1] == '?' {
		return nodeName[:nameLen-1], true
	}
	return nodeName, false
}

// makeProperty creates a property from a YAML node.
func (r *RAML) makeProperty(nodeName string, propertyName string, v *yaml.Node,
	location string, hasImplicitOptional bool) (Property, error) {
	shape, err := r.makeNewShapeYAML(v, nodeName, location)
	if err != nil {
		return Property{}, StacktraceNewWrapped("make shape", err, location, WithNodePosition(v))
	}
	finalName := propertyName
	var required bool
	shapeRequired := shape.Required
	if shapeRequired == nil {
		// If shape has no "required" facet, requirement depends only on whether "?"" was used in node name.
		required = !hasImplicitOptional
	} else {
		// If shape explicitly defines "required" facet combined with "?" in node name - explicit
		// definition prevails and property name keeps the node name.
		// Otherwise, keep propertyName that has the last "?" chomped.
		if hasImplicitOptional {
			finalName = nodeName
		}
		required = *shapeRequired
	}
	return Property{
		Name:     finalName,
		Shape:    shape,
		Required: required,
		raml:     r,
	}, nil
}

// Property represents a property of an object shape.
type Property struct {
	Name     string
	Shape    *BaseShape
	Required bool
	raml     *RAML
}

// Property represents a pattern property of an object shape.
type PatternProperty struct {
	Pattern *regexp.Regexp
	Shape   *BaseShape
	// Pattern properties are always optional.
	raml *RAML
}

// UnionFacets contains constraints for union shapes.
type UnionFacets struct {
	AnyOf []*BaseShape
}

// UnionShape represents a union shape.
type UnionShape struct {
	*BaseShape

	EnumFacets
	UnionFacets
}

// UnmarshalYAMLNodes unmarshals the union shape from YAML nodes.
func (s *UnionShape) unmarshalYAMLNodes(_ []*yaml.Node) error {
	return nil
}

// Base returns the base shape.
func (s *UnionShape) Base() *BaseShape {
	return s.BaseShape
}

func (s *UnionShape) clone(base *BaseShape, clonedMap map[int64]*BaseShape) Shape {
	c := *s
	c.BaseShape = base
	c.AnyOf = make([]*BaseShape, len(s.AnyOf))
	for i, member := range s.AnyOf {
		c.AnyOf[i] = member.clone(clonedMap)
	}
	return &c
}

func (s *UnionShape) validate(v interface{}, ctxPath string) error {
	// TODO: Collect errors
	for _, item := range s.AnyOf {
		if err := item.Shape.validate(v, ctxPath); err == nil {
			return nil
		}
	}
	return stacktrace.New("value does not match any type", s.Location,
		stacktrace.WithPosition(&s.Position))
}

// Inherit merges the source shape into the target shape.
func (s *UnionShape) inherit(source Shape) (Shape, error) {
	ss, ok := source.(*UnionShape)
	if !ok {
		return nil, stacktrace.New("cannot inherit from different type", s.Location,
			stacktrace.WithPosition(&s.Position),
			stacktrace.WithInfo("source", source.Base().Type),
			stacktrace.WithInfo("target", s.Base().Type))
	}
	if len(s.AnyOf) == 0 {
		s.AnyOf = ss.AnyOf
		return s, nil
	}
	// TODO: Implement enum facets inheritance
	var finalFiltered []*BaseShape
	for _, sourceMember := range ss.AnyOf {
		var filtered []*BaseShape
		for _, targetMember := range s.AnyOf {
			if sourceMember.Type == targetMember.Type {
				// Clone is required to avoid modifying the original target member shape.
				cs := targetMember.CloneDetached()
				// TODO: Probably all copied shapes must change IDs since these are actually new shapes.
				cs.ID = generateShapeID()
				ms, err := cs.Inherit(sourceMember)
				if err != nil {
					// TODO: Collect errors
					// StacktraceNewWrapped("merge union member", err, s.Location)
					continue
				}
				filtered = append(filtered, ms)
			}
		}
		if len(filtered) == 0 {
			return nil, stacktrace.New("failed to find compatible union member", s.Location,
				stacktrace.WithPosition(&s.Position))
		}
		finalFiltered = append(finalFiltered, filtered...)
	}
	s.AnyOf = finalFiltered
	return s, nil
}

func (s *UnionShape) check() error {
	for _, item := range s.AnyOf {
		if err := item.Check(); err != nil {
			return StacktraceNewWrapped("check union member", err, s.Location,
				stacktrace.WithPosition(&item.Position))
		}
	}
	return nil
}

type JSONShape struct {
	*BaseShape

	Schema *JSONSchema
	Raw    string
}

func (s *JSONShape) Base() *BaseShape {
	return s.BaseShape
}

func (s *JSONShape) clone(base *BaseShape, _ map[int64]*BaseShape) Shape {
	c := *s
	c.BaseShape = base
	return &c
}

func (s *JSONShape) validate(_ interface{}, _ string) error {
	// TODO: Implement validation with JSON Schema
	return nil
}

func (s *JSONShape) unmarshalYAMLNodes(_ []*yaml.Node) error {
	return nil
}

func (s *JSONShape) inherit(source Shape) (Shape, error) {
	ss, ok := source.(*JSONShape)
	if !ok {
		return nil, stacktrace.New("cannot inherit from different type", s.Location,
			stacktrace.WithPosition(&s.Position), stacktrace.WithInfo("source", source.Base().Type),
			stacktrace.WithInfo("target", s.Base().Type))
	}
	if s.Raw != "" && ss.Raw != "" && s.Raw != ss.Raw {
		return nil, stacktrace.New("cannot inherit from different JSON schema", s.Location,
			stacktrace.WithPosition(&s.Position))
	}
	s.Schema = ss.Schema
	s.Raw = ss.Raw
	return s, nil
}

func (s *JSONShape) check() error {
	// TODO: JSON Schema check
	return nil
}

type UnknownShape struct {
	*BaseShape

	facets []*yaml.Node
}

func (s *UnknownShape) Base() *BaseShape {
	return s.BaseShape
}

func (s *UnknownShape) clone(base *BaseShape, _ map[int64]*BaseShape) Shape {
	c := *s
	c.BaseShape = base
	return &c
}

func (s *UnknownShape) validate(_ interface{}, _ string) error {
	return stacktrace.New("cannot validate against unknown shape", s.Location, stacktrace.WithPosition(&s.Position))
}

func (s *UnknownShape) unmarshalYAMLNodes(v []*yaml.Node) error {
	s.facets = v
	return nil
}

func (s *UnknownShape) inherit(_ Shape) (Shape, error) {
	return nil, stacktrace.New("cannot inherit from unknown shape", s.Location, stacktrace.WithPosition(&s.Position))
}

func (s *UnknownShape) check() error {
	return stacktrace.New("cannot check unknown shape", s.Location, stacktrace.WithPosition(&s.Position))
}

type RecursiveShape struct {
	*BaseShape

	Head *BaseShape
}

func (s *RecursiveShape) unmarshalYAMLNodes(_ []*yaml.Node) error {
	return nil
}

func (s *RecursiveShape) Base() *BaseShape {
	return s.BaseShape
}

func (s *RecursiveShape) clone(base *BaseShape, _ map[int64]*BaseShape) Shape {
	c := *s
	c.BaseShape = base
	return &c
}

func (s *RecursiveShape) validate(v interface{}, ctxPath string) error {
	if err := s.Head.Shape.validate(v, ctxPath); err != nil {
		return fmt.Errorf("validate recursive shape: %w", err)
	}
	return nil
}

// Inherit merges the source shape into the target shape.
func (s *RecursiveShape) inherit(_ Shape) (Shape, error) {
	return nil, stacktrace.New("cannot inherit from recursive shape", s.Location, stacktrace.WithPosition(&s.Position))
}

func (s *RecursiveShape) check() error {
	return nil
}
