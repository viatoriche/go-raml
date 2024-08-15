package raml

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type ArrayFacets struct {
	Items       *Shape
	MinItems    *any
	MaxItems    *any
	UniqueItems *bool
}

type ArrayShape struct {
	BaseShape

	ArrayFacets
}

func (s *ArrayShape) Base() *BaseShape {
	return &s.BaseShape
}

func (s *ArrayShape) Clone() Shape {
	c := *s
	return &c
}

func (s *ArrayShape) UnmarshalYAMLNodes(v []*yaml.Node) error {
	for i := 0; i != len(v); i += 2 {
		node := v[i]
		valueNode := v[i+1]

		if node.Value == "minItems" {
			if err := valueNode.Decode(&s.MinItems); err != nil {
				return fmt.Errorf("decode minItems: %w", err)
			}
		} else if node.Value == "maxItems" {
			if err := valueNode.Decode(&s.MaxItems); err != nil {
				return fmt.Errorf("decode maxItems: %w", err)
			}
		} else if node.Value == "items" {
			name := "items"
			shape, err := MakeShape(valueNode, name, s.Location)
			if err != nil {
				return fmt.Errorf("make shape: %w", err)
			}
			s.Items = shape
			GetRegistry().PutIntoFragment(s.Name+"#items", s.Location, s.Items)
		} else if node.Value == "uniqueItems" {
			if err := valueNode.Decode(&s.UniqueItems); err != nil {
				return fmt.Errorf("decode uniqueItems: %w", err)
			}
		} else {
			dt, err := MakeNode(valueNode, s.Location)
			if err != nil {
				return fmt.Errorf("make node: %w", err)
			}
			s.CustomShapeFacets[node.Value] = dt
		}
	}
	return nil
}

type ObjectFacets struct {
	Discriminator        string
	DiscriminatorValue   any
	AdditionalProperties bool
	Properties           map[string]*Property
	MinProperties        *any
	MaxProperties        *any
}

type ObjectShape struct {
	BaseShape

	ObjectFacets
}

func (s *ObjectShape) UnmarshalYAMLNodes(v []*yaml.Node) error {
	s.AdditionalProperties = true // Additional properties is true by default

	for i := 0; i != len(v); i += 2 {
		node := v[i]
		valueNode := v[i+1]

		if node.Value == "additionalProperties" {
			if err := valueNode.Decode(&s.AdditionalProperties); err != nil {
				return fmt.Errorf("decode additionalProperties: %w", err)
			}
		} else if node.Value == "discriminator" {
			if err := valueNode.Decode(&s.Discriminator); err != nil {
				return fmt.Errorf("decode discriminator: %w", err)
			}
		} else if node.Value == "discriminatorValue" {
			if err := valueNode.Decode(&s.DiscriminatorValue); err != nil {
				return fmt.Errorf("decode discriminatorValue: %w", err)
			}
		} else if node.Value == "minProperties" {
			if err := valueNode.Decode(&s.MinProperties); err != nil {
				return fmt.Errorf("decode minProperties: %w", err)
			}
		} else if node.Value == "maxProperties" {
			if err := valueNode.Decode(&s.MaxProperties); err != nil {
				return fmt.Errorf("decode maxProperties: %w", err)
			}
		} else if node.Value == "properties" {
			s.Properties = make(map[string]*Property)
			for j := 0; j != len(valueNode.Content); j += 2 {
				name := valueNode.Content[j].Value
				data := valueNode.Content[j+1]
				property, err := MakeProperty(name, data, s.Location)
				if err != nil {
					return fmt.Errorf("make property: %w", err)
				}
				s.Properties[property.Name] = property
				GetRegistry().PutIntoFragment(s.Name+"#"+property.Name, s.Location, property.Shape)
			}
		} else {
			dt, err := MakeNode(valueNode, s.Location)
			if err != nil {
				return fmt.Errorf("make node: %w", err)
			}
			s.CustomShapeFacets[node.Value] = dt
		}
	}
	return nil
}

func (s *ObjectShape) Base() *BaseShape {
	return &s.BaseShape
}

func (s *ObjectShape) Clone() Shape {
	c := *s
	return &c
}

func MakeProperty(name string, v *yaml.Node, location string) (*Property, error) {
	shape, err := MakeShape(v, name, location)
	if err != nil {
		return nil, fmt.Errorf("make shape: %w", err)
	}
	propertyName := name
	shapeRequired := (*shape).Base().Required
	var required bool
	if shapeRequired == nil {
		if strings.HasSuffix(propertyName, "?") {
			required = false
			propertyName = propertyName[:len(propertyName)-1]
		} else {
			required = true
		}
	} else {
		required = *shapeRequired
	}
	return &Property{
		Name:     propertyName,
		Shape:    shape,
		Required: required,
	}, nil
}

type Property struct {
	Name     string
	Shape    *Shape
	Required bool
}

type UnionFacets struct {
	AnyOf []*Shape
}

type UnionShape struct {
	BaseShape

	EnumFacets
	UnionFacets
}

func (s *UnionShape) UnmarshalYAMLNodes(values []*yaml.Node) error {
	return nil
}

func (s *UnionShape) Base() *BaseShape {
	return &s.BaseShape
}

func (s *UnionShape) Clone() Shape {
	c := *s
	return &c
}

type JSONShape struct {
	BaseShape
}

func (s *JSONShape) Base() *BaseShape {
	return &s.BaseShape
}

func (s *JSONShape) Clone() Shape {
	c := *s
	return &c
}

func (s *JSONShape) UnmarshalYAMLNodes(v []*yaml.Node) error {
	return nil
}

type UnknownShape struct {
	BaseShape

	facets []*yaml.Node
}

func (s *UnknownShape) Base() *BaseShape {
	return &s.BaseShape
}

func (s *UnknownShape) Clone() Shape {
	c := *s
	return &c
}

func (s *UnknownShape) UnmarshalYAMLNodes(v []*yaml.Node) error {
	s.facets = v
	return nil
}