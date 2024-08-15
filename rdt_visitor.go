package raml

import (
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"gopkg.in/yaml.v3"

	"github.com/acronis/go-raml/rdt"
)

// Define a struct that implements the visitor
type RdtVisitor struct {
	rdt.BaserdtParserVisitor // Embedding the base visitor class
}

func NewRdtVisitor() *RdtVisitor {
	return &RdtVisitor{}
}

func (visitor *RdtVisitor) Visit(tree antlr.ParseTree, target *UnknownShape) (*Shape, error) {
	// Target is required to isolate anonymous shapes created by Union, Optional and Array syntax.
	// This is done to avoid sharing base shape properties between the original type and implicitly created type.
	switch t := tree.(type) {
	case *rdt.EntrypointContext:
		return visitor.VisitEntrypoint(t, target)
	case *rdt.ExpressionContext:
		return visitor.VisitExpression(t, target)
	case *rdt.TypeContext:
		return visitor.VisitType(t, target)
	case *rdt.PrimitiveContext:
		return visitor.VisitPrimitive(t, target)
	case *rdt.OptionalContext:
		return visitor.VisitOptional(t, target)
	case *rdt.ArrayContext:
		return visitor.VisitArray(t, target)
	case *rdt.UnionContext:
		return visitor.VisitUnion(t, target)
	case *rdt.GroupContext:
		return visitor.VisitGroup(t, target)
	case *rdt.ReferenceContext:
		return visitor.VisitReference(t, target)
	}
	return nil, fmt.Errorf("unknown node type %T", tree)
}

func (visitor *RdtVisitor) VisitChildren(node antlr.RuleNode, target *UnknownShape) ([]*Shape, error) {
	var shapes []*Shape
	for _, n := range node.GetChildren() {
		// Skip terminal nodes
		if _, ok := n.(*antlr.TerminalNodeImpl); ok {
			continue
		}
		implicitAnonShape := &UnknownShape{BaseShape: *MakeBaseShape("", target.Base().Location, &Position{})}
		s, err := visitor.Visit(n.(antlr.ParseTree), implicitAnonShape)
		if err != nil {
			return nil, err
		}
		shapes = append(shapes, s)
	}
	return shapes, nil
}

func (visitor *RdtVisitor) VisitEntrypoint(ctx *rdt.EntrypointContext, target *UnknownShape) (*Shape, error) {
	return visitor.Visit(ctx.GetChildren()[0].(antlr.ParseTree), target)
}

func (visitor *RdtVisitor) VisitExpression(ctx *rdt.ExpressionContext, target *UnknownShape) (*Shape, error) {
	return visitor.Visit(ctx.GetChildren()[0].(antlr.ParseTree), target)
}

func (visitor *RdtVisitor) VisitType(ctx *rdt.TypeContext, target *UnknownShape) (*Shape, error) {
	return visitor.Visit(ctx.GetChildren()[0].(antlr.ParseTree), target)
}

func (visitor *RdtVisitor) VisitPrimitive(ctx *rdt.PrimitiveContext, target *UnknownShape) (*Shape, error) {
	s, err := MakeConcreteShape(target.Base(), ctx.GetText(), make([]*yaml.Node, 0))
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (visitor *RdtVisitor) VisitOptional(ctx *rdt.OptionalContext, target *UnknownShape) (*Shape, error) {
	implicitAnonShape := &UnknownShape{BaseShape: *MakeBaseShape("", target.Base().Location, &Position{})}
	s, err := visitor.Visit(ctx.GetChildren()[0].(antlr.ParseTree), implicitAnonShape)
	if err != nil {
		return nil, err
	}
	base := target.Base()
	base.Type = TypeUnion
	// Nil shape is also anonymous here and doesn't share the base shape with the target.
	nilShape, _ := MakeConcreteShape(&BaseShape{Location: base.Location}, "nil", make([]*yaml.Node, 0))
	var unionShape Shape = &UnionShape{
		BaseShape: *base,
		UnionFacets: UnionFacets{
			AnyOf: []*Shape{s, &nilShape},
		},
	}
	return &unionShape, nil
}

func (visitor *RdtVisitor) VisitArray(ctx *rdt.ArrayContext, target *UnknownShape) (*Shape, error) {
	implicitAnonShape := &UnknownShape{BaseShape: *MakeBaseShape("", target.Base().Location, &Position{})}
	s, err := visitor.Visit(ctx.GetChildren()[0].(antlr.ParseTree), implicitAnonShape)
	if err != nil {
		return nil, err
	}
	base := target.Base()
	base.Type = TypeArray
	var arrayShape Shape = &ArrayShape{
		BaseShape: *base,
		ArrayFacets: ArrayFacets{
			Items: s,
		},
	}
	return &arrayShape, nil
}

func (visitor *RdtVisitor) VisitUnion(ctx *rdt.UnionContext, target *UnknownShape) (*Shape, error) {
	ss, err := visitor.VisitChildren(ctx, target)
	if err != nil {
		return nil, err
	}
	base := target.Base()
	base.Type = TypeUnion
	var unionShape Shape = &UnionShape{
		BaseShape: *base,
		UnionFacets: UnionFacets{
			AnyOf: ss,
		},
	}
	return &unionShape, nil
}

func (visitor *RdtVisitor) VisitGroup(ctx *rdt.GroupContext, target *UnknownShape) (*Shape, error) {
	return visitor.Visit(ctx.GetChildren()[0].(antlr.ParseTree), target)
}

func (visitor *RdtVisitor) VisitReference(ctx *rdt.ReferenceContext, target *UnknownShape) (*Shape, error) {
	// TODO: In theory, this can be not only library so this type assertion may fail.
	frag := GetRegistry().GetFragment(target.Base().Location).(*Library)

	// External ref - lib.Type
	// Internal ref - Type
	shapeType := ctx.GetText()
	parts := strings.Split(shapeType, ".")
	var ref *Shape
	if len(parts) == 1 {
		ref = frag.Types[parts[0]]
		if ref == nil {
			return nil, fmt.Errorf("reference %s not found", parts[0])
		}
	} else if len(parts) == 2 {
		lib := frag.Uses[parts[0]]
		if lib == nil {
			return nil, fmt.Errorf("library %s not found", parts[0])
		}
		ref = lib.Types[parts[1]]
		if ref == nil {
			return nil, fmt.Errorf("reference %s not found", parts[1])
		}
	} else {
		return nil, fmt.Errorf("invalid reference %s", shapeType)
	}
	if err := Resolve(ref); err != nil {
		return nil, err
	}
	s, err := MakeConcreteShape(target.Base(), (*ref).Base().Type, target.facets)
	if err != nil {
		return nil, err
	}
	s.Base().Inherits = append(target.Base().Inherits, ref)
	return &s, nil
}
