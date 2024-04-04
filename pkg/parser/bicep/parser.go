package bicep

import (
	"encoding/json"
	"fmt"
	"strings"
	"strconv"

	"github.com/Checkmarx/kics/pkg/model"
	"github.com/Checkmarx/kics/pkg/parser/bicep/antlr/parser"
	"github.com/antlr4-go/antlr/v4"
)

type Parser struct {
}

type BicepVisitor struct {
	parser.BasebicepVisitor
	paramList    map[string]interface{}
	varList      map[string]interface{}
	resourceList []interface{}
}

type JSONBicep struct {
	Parameters map[string]interface{} `json:"parameters"`
	Variables  map[string]interface{} `json:"variables"`
	Resources  []interface{}          `json:"resources"`
}

func NewBicepVisitor() *BicepVisitor {
	paramList := map[string]interface{}{}
	varList := map[string]interface{}{}
	resourceList := []interface{}{}
	return &BicepVisitor{paramList: paramList, varList: varList, resourceList: resourceList}
}

func convertVisitorToJSONBicep(visitor *BicepVisitor) *JSONBicep {
	return &JSONBicep{
		Parameters: visitor.paramList,
		Variables:  visitor.varList,
		Resources:  visitor.resourceList,
	}
}

// Parse - parses bicep to BicepVisitor template (json file)
func (p *Parser) Parse(file string, _ []byte) ([]model.Document, []int, error) {
	fmt.Println(file)
	bicepVisitor := NewBicepVisitor()
	stream, _ := antlr.NewFileStream(file)
	lexer := parser.NewbicepLexer(stream)

	tokenStream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	bicepParser := parser.NewbicepParser(tokenStream)
	bicepParser.RemoveErrorListeners()
	bicepParser.AddErrorListener(antlr.NewDiagnosticErrorListener(true))

	bicepParser.Program().Accept(bicepVisitor)
	fmt.Println("\nParameters: ", bicepVisitor.paramList)
	fmt.Println("\nVariables: ", bicepVisitor.varList)
	fmt.Println("\nResources: ", bicepVisitor.resourceList)

	var doc model.Document

	jBicep := convertVisitorToJSONBicep(bicepVisitor)
	bicepBytes, err := json.Marshal(jBicep)
	if err != nil {
		return nil, nil, err
	}

	err = json.Unmarshal(bicepBytes, &doc)
	if err != nil {
		return nil, nil, err
	}

	return []model.Document{doc}, nil, nil
}

func (v *BicepVisitor) VisitProgram(ctx *parser.ProgramContext) interface{} {
	for _, val := range ctx.AllStatement() {
		val.Accept(v)
	}

	return nil
}

func (s *BicepVisitor) VisitStatement(ctx *parser.StatementContext) interface{} {

	if ctx.ParameterDecl() != nil {
		return ctx.ParameterDecl().Accept(s)
	}
	if ctx.VariableDecl() != nil {
		return ctx.VariableDecl().Accept(s)
	}
	if ctx.ResourceDecl() != nil {
		return ctx.ResourceDecl().Accept(s)
	}

	return nil
}

// VisitParameterDecl is called when production paramDecl is visited.
func (s *BicepVisitor) VisitParameterDecl(ctx *parser.ParameterDeclContext) interface{} {
	var decorators []interface{}
	param := map[string]interface{}{}
	identifier := ctx.Identifier().Accept(s)
	if ctx.ParameterDefaultValue() != nil {
		paramVal := ctx.ParameterDefaultValue().Accept(s)
		param["value"] = paramVal
	}
	if ctx.TypeExpression() != nil {
		typeExpression := ctx.TypeExpression().Accept(s)
		param["type"] = typeExpression
	}

	for _, val := range ctx.AllDecorator() {
		decorators = append(decorators, val.Accept(s))
	}
	param["decorators"] = decorators
	s.paramList[identifier.(string)] = param
	return nil
}

// VisitParameterDecl is called when production paramDecl is visited.
func (s *BicepVisitor) VisitVariableDecl(ctx *parser.VariableDeclContext) interface{} {
	var variable = map[string]interface{}{}
	var decorators []interface{}
	identifier := ctx.Identifier().Accept(s)
	expression := ctx.Expression().Accept(s)

	for _, val := range ctx.AllDecorator() {
		decorators = append(decorators, val.Accept(s))
	}
	variable["decorators"] = decorators
	variable["value"] = expression
	s.varList[identifier.(string)] = variable

	return nil
}

func (s *BicepVisitor) VisitResourceDecl(ctx *parser.ResourceDeclContext) interface{} {
	resource := map[string]interface{}{}
	var decorators []interface{}
	interpString := ctx.InterpString().Accept(s)
	identifier := ctx.Identifier().Accept(s)
	resourceType := strings.Split(interpString.(string), "@")[0]
	apiVersion := strings.Split(interpString.(string), "@")[1]
	resource["type"] = resourceType
	resource["apiVersion"] = apiVersion
	for _, val := range ctx.AllDecorator() {
		decorators = append(decorators, val.Accept(s))
	}
	resource["decorators"] = decorators
	resource["name"] = identifier
	if ctx.Object() != nil {
		object := ctx.Object().Accept(s)
		for key, val := range object.(map[string]interface{}) {
			resource[key] = val
		}
	}

	s.resourceList = append(s.resourceList, resource)

	return nil
}

// VisitParameterDefaultValue is called when production paramDecl is visited.
func (s *BicepVisitor) VisitParameterDefaultValue(ctx *parser.ParameterDefaultValueContext) interface{} {
	param := ctx.Expression().Accept(s)
	return param
}

func (s *BicepVisitor) VisitExpression(ctx *parser.ExpressionContext) interface{} {
	if ctx.GetChildCount() > 1 {
		if ctx.Identifier() != nil {
			identifier := ctx.Identifier().Accept(s)
			exp := ctx.Expression(0).Accept(s)
			fmt.Println("Visit Expression value: ", exp)
			if ctx.DOT() != nil {
				return identifier.(string)
			}

			return nil
		} else {
			for _, val := range ctx.AllExpression() {
				val.Accept(s)
			}
		}
	}

	return ctx.PrimaryExpression().Accept(s)
}

// VisitPrimaryExpression is called when production primaryExpression is visited.
func (s *BicepVisitor) VisitPrimaryExpression(ctx *parser.PrimaryExpressionContext) interface{} {
	if ctx.LiteralValue() != nil {
		return ctx.LiteralValue().Accept(s)
	}
	if ctx.FunctionCall() != nil {
		return ctx.FunctionCall().Accept(s)
	}
	if ctx.InterpString() != nil {
		return ctx.InterpString().Accept(s)
	}
	if ctx.MULTILINE_STRING() != nil {
		return ctx.MULTILINE_STRING().GetText()
	}
	if ctx.Array() != nil {
		return ctx.Array().Accept(s)
	}
	if ctx.Object() != nil {
		return ctx.Object().Accept(s)
	}
	if ctx.ParenthesizedExpression() != nil {
		return ctx.ParenthesizedExpression().Accept(s)
	}

	return nil
}

func (s *BicepVisitor) VisitLiteralValue(ctx *parser.LiteralValueContext) interface{} {
	if ctx.NUMBER() != nil {
		number, _ := strconv.ParseFloat(ctx.NUMBER().GetText(), 32)
		return number
	}
	if ctx.TRUE() != nil {
		return true
	}
	if ctx.FALSE() != nil {
		return false
	}
	if ctx.NULL() != nil {
		return nil
	}
	if ctx.Identifier() != nil {
		return ctx.Identifier().Accept(s)
	}

	return nil
}

// VisitInterpString is called when production interpString is visited.
func (s *BicepVisitor) VisitInterpString(ctx *parser.InterpStringContext) interface{} {
	if ctx.GetChildCount() > 1 {
		interpString := []interface{}{}
		interpString = append(interpString, ctx.STRING_LEFT_PIECE().GetText())
		if ctx.AllSTRING_MIDDLE_PIECE() != nil && (len(ctx.AllSTRING_MIDDLE_PIECE()) > 0) {
			for idx, val := range ctx.AllSTRING_MIDDLE_PIECE() {
				interpString = append(interpString, ctx.Expression(idx).Accept(s))
				interpString = append(interpString, val.GetText())
			}
		}
		// Last expression with string right piece
		interpString = append(interpString, ctx.Expression(len(ctx.AllSTRING_MIDDLE_PIECE())).Accept(s))
		interpString = append(interpString, ctx.STRING_RIGHT_PIECE().GetText())
		str := ""
		for _, v := range interpString {
			switch v := v.(type) {
			case (string):
				str = str + v
			case (map[string][]interface{}):
				for identifier, argumentList := range v {
					resStr := "[" + identifier + "("
					for idx, arg := range argumentList {
						resStr += arg.(string)
						if idx < len(argumentList)-1 {
							resStr += ", "
						}
					}

					resStr += ")]"
					str += resStr
				}
			}

		}
		return str
	}

	unformattedString := ctx.STRING_COMPLETE().GetText()
	finalString := strings.ReplaceAll(unformattedString, "'", "")
	return finalString
}

func (s *BicepVisitor) VisitArray(ctx *parser.ArrayContext) interface{} {
	array := []interface{}{}
	for _, val := range ctx.AllArrayItem() {
		expression := val.Accept(s)
		array = append(array, expression)
	}
	return array
}

func (s *BicepVisitor) VisitArrayItem(ctx *parser.ArrayItemContext) interface{} {
	return ctx.Expression().Accept(s)
}

func (s *BicepVisitor) VisitObject(ctx *parser.ObjectContext) interface{} {
	object := map[string]interface{}{}
	for _, val := range ctx.AllObjectProperty() {
		objectProperty := val.Accept(s).(map[string]interface{})
		for key, val := range objectProperty {
			object[key] = val
		}
	}

	return object
}

func (s *BicepVisitor) VisitObjectProperty(ctx *parser.ObjectPropertyContext) interface{} {
	objectValue := ctx.Expression().Accept(s)
	objectProperty := map[string]interface{}{}
	if ctx.Identifier() != nil {
		identifier := ctx.Identifier().Accept(s)
		objectProperty[identifier.(string)] = ctx.Expression().Accept(s)
	}
	if ctx.InterpString() != nil {
		interpString := ctx.InterpString().Accept(s)
		objectProperty[interpString.(string)] = objectValue
	}

	return objectProperty
}

func (s *BicepVisitor) VisitIdentifier(ctx *parser.IdentifierContext) interface{} {
	if ctx.IDENTIFIER() != nil {
		return ctx.IDENTIFIER().GetText()
	}
	if (ctx.PARAM()) != nil {
		return ctx.PARAM().GetText()
	}
	if (ctx.RESOURCE()) != nil {
		return ctx.RESOURCE().GetText()
	}
	if (ctx.VAR()) != nil {
		return ctx.VAR().GetText()
	}
	if (ctx.TRUE()) != nil {
		return ctx.TRUE().GetText()
	}
	if (ctx.FALSE()) != nil {
		return ctx.FALSE().GetText()
	}
	if (ctx.NULL()) != nil {
		return ctx.NULL().GetText()
	}
	if (ctx.STRING()) != nil {
		return ctx.STRING().GetText()
	}
	if (ctx.INT()) != nil {
		return ctx.INT().GetText()
	}
	if (ctx.BOOL()) != nil {
		return ctx.BOOL().GetText()
	}
	return nil
}

func (s *BicepVisitor) VisitParenthesizedExpression(ctx *parser.ParenthesizedExpressionContext) interface{} {
	return ctx.Expression().Accept(s)
}

func (s *BicepVisitor) VisitDecorator(ctx *parser.DecoratorContext) interface{} {
	decorator := ctx.DecoratorExpression().Accept(s)
	return decorator
}

func (s *BicepVisitor) VisitDecoratorExpression(ctx *parser.DecoratorExpressionContext) interface{} {
	return ctx.FunctionCall().Accept(s)
}

func (s *BicepVisitor) VisitFunctionCall(ctx *parser.FunctionCallContext) interface{} {
	identifier := ctx.Identifier().Accept(s)
	var argumentList []interface{}
	if ctx.ArgumentList() != nil {
		argumentList = ctx.ArgumentList().Accept(s).([]interface{})
	}
	functionCall := map[string]interface{}{
		identifier.(string): argumentList,
	}

	return functionCall
}

func (s *BicepVisitor) VisitArgumentList(ctx *parser.ArgumentListContext) interface{} {
	var argumentList []interface{}
	for _, val := range ctx.AllExpression() {
		argument := val.Accept(s)
		argumentList = append(argumentList, argument)
	}
	return argumentList
}

func (s *BicepVisitor) VisitTypeExpression(ctx *parser.TypeExpressionContext) interface{} {
	identifiers := []string{}
	for _, val := range ctx.AllIdentifier() {
		identifiers = append(identifiers, val.Accept(s).(string))
	}
	return identifiers
}

// GetKind returns the kind of the parser
func (p *Parser) GetKind() model.FileKind {
	return model.KindBICEP
}

// SupportedExtensions returns Bicep extensions
func (p *Parser) SupportedExtensions() []string {
	return []string{".bicep"}
}

// SupportedTypes returns types supported by this parser, which are bicep files
func (p *Parser) SupportedTypes() map[string]bool {
	return map[string]bool{"bicep": true, "azureresourcemanager": true}
}

// GetCommentToken return the comment token of Bicep files - #
func (p *Parser) GetCommentToken() string {
	return "//"
}

// StringifyContent converts original content into string formatted version
func (p *Parser) StringifyContent(content []byte) (string, error) {
	return string(content), nil
}

// Resolve resolves bicep files variables
func (p *Parser) Resolve(fileContent []byte, _ string, _ bool) ([]byte, error) {
	return fileContent, nil
}

// GetResolvedFiles returns the list of files that are resolved
func (p *Parser) GetResolvedFiles() map[string]model.ResolvedFile {
	return make(map[string]model.ResolvedFile)
}
