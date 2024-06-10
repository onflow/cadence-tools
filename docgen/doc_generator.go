/*
 * Cadence docgen - The Cadence documentation generator
 *
 * Copyright Flow Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package docgen

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"text/template"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/parser"

	"github.com/onflow/cadence-tools/docgen/templates"
)

const nameSeparator = "_"
const newline = "\n"
const mdFileExt = ".md"
const paramPrefix = "@param "
const returnPrefix = "@return "

const baseTemplate = "base-template"
const compositeFullTemplate = "composite-full-template"

var templateFiles = []string{
	baseTemplate,
	compositeFullTemplate,
	"composite-members-template",
	"function-template",
	"composite-template",
	"field-template",
	"enum-template",
	"enum-case-template",
	"initializer-template",
	"event-template",
}

type DocGenerator struct {
	entryPageGen     *template.Template
	compositePageGen *template.Template
	typeNames        []string
	outputDir        string
	files            InMemoryFiles
}

type InMemoryFiles map[string][]byte

type InMemoryFileWriter struct {
	fileName string
	buf      *bytes.Buffer
	files    InMemoryFiles
}

func NewInMemoryFileWriter(files InMemoryFiles, fileName string) *InMemoryFileWriter {
	return &InMemoryFileWriter{
		fileName: fileName,
		files:    files,
		buf:      &bytes.Buffer{},
	}
}

func (w *InMemoryFileWriter) Write(bytes []byte) (n int, err error) {
	return w.buf.Write(bytes)
}

func (w *InMemoryFileWriter) Close() error {
	w.files[w.fileName] = w.buf.Bytes()
	w.buf = nil
	return nil
}

func NewDocGenerator() *DocGenerator {
	gen := &DocGenerator{}

	functions := newTemplateFunctions[ast.Declaration](ASTDeclarationTemplateFunctions{})

	functions["fileName"] = func(decl ast.Declaration) string {
		fileNamePrefix := gen.currentFileName()
		if len(fileNamePrefix) == 0 {
			return fmt.Sprint(decl.DeclarationIdentifier().String(), mdFileExt)
		}

		return fmt.Sprint(fileNamePrefix, nameSeparator, decl.DeclarationIdentifier().String(), mdFileExt)
	}

	templateProvider := templates.NewMarkdownTemplateProvider()

	gen.entryPageGen = newTemplate(baseTemplate, templateProvider, functions)
	gen.compositePageGen = newTemplate(compositeFullTemplate, templateProvider, functions)

	return gen
}

func newTemplate(name string, templateProvider templates.TemplateProvider, functions template.FuncMap) *template.Template {
	rootTemplate := template.New(name).Funcs(functions)

	for _, templateFile := range templateFiles {
		content, err := templateProvider.Get(templateFile)
		if err != nil {
			panic(err)
		}

		var tmpl *template.Template
		if templateFile == name {
			tmpl = rootTemplate
		} else {
			tmpl = rootTemplate.New(name)
		}

		_, err = tmpl.Parse(content)
		if err != nil {
			panic(err)
		}
	}

	return rootTemplate
}

func (gen *DocGenerator) Generate(source string, outputDir string) error {
	gen.outputDir = outputDir
	gen.typeNames = nil

	program, err := parser.ParseProgram([]byte(source), nil)
	if err != nil {
		return err
	}

	return gen.genProgram(program)
}

func (gen *DocGenerator) GenerateInMemory(source string) (InMemoryFiles, error) {
	gen.files = InMemoryFiles{}
	gen.typeNames = nil

	program, err := parser.ParseProgram([]byte(source), nil)
	if err != nil {
		return nil, err
	}

	err = gen.genProgram(program)
	if err != nil {
		return nil, err
	}

	return gen.files, nil
}

func (gen *DocGenerator) genProgram(program *ast.Program) error {

	// If the program does not have a sole declaration,
	// i.e. it has multiple top level declarations,
	// then generate an entry page.

	if program.SoleContractDeclaration() == nil &&
		program.SoleContractInterfaceDeclaration() == nil {

		// Generate entry page
		// TODO: file name 'index' can conflict with struct names, resulting an overwrite.
		f, err := gen.fileWriter("index.md")
		if err != nil {
			return err
		}
		defer f.Close()

		err = gen.entryPageGen.Execute(f, program)
		if err != nil {
			return err
		}
	}

	// Generate dedicated pages for all the nested composite declarations
	return gen.genDeclarations(program.Declarations())
}

func (gen *DocGenerator) genDeclarations(decls []ast.Declaration) error {
	var err error
	for _, decl := range decls {
		switch astDecl := decl.(type) {
		case *ast.CompositeDeclaration:
			err = gen.genCompositeDeclaration(astDecl)
		case *ast.InterfaceDeclaration:
			err = gen.genInterfaceDeclaration(astDecl)
		default:
			// do nothing
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (gen *DocGenerator) genCompositeDeclaration(declaration *ast.CompositeDeclaration) error {
	if declaration.DeclarationKind() == common.DeclarationKindEvent {
		return nil
	}

	declName := declaration.DeclarationIdentifier().String()
	return gen.genCompositeDecl(declName, declaration.Members, declaration)
}

func (gen *DocGenerator) genInterfaceDeclaration(declaration *ast.InterfaceDeclaration) error {
	declName := declaration.DeclarationIdentifier().String()
	return gen.genCompositeDecl(declName, declaration.Members, declaration)
}

func (gen *DocGenerator) genCompositeDecl(name string, members *ast.Members, decl ast.Declaration) error {
	gen.typeNames = append(gen.typeNames, name)
	defer func() {
		lastIndex := len(gen.typeNames) - 1
		gen.typeNames = gen.typeNames[:lastIndex]
	}()

	fileName := fmt.Sprint(gen.currentFileName(), mdFileExt)
	f, err := gen.fileWriter(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	err = gen.compositePageGen.Execute(f, decl)
	if err != nil {
		return err
	}

	return gen.genDeclarations(members.Declarations())
}

func (gen *DocGenerator) fileWriter(fileName string) (io.WriteCloser, error) {
	if gen.files == nil {
		return os.Create(path.Join(gen.outputDir, fileName))
	}

	return NewInMemoryFileWriter(gen.files, fileName), nil
}

func (gen *DocGenerator) currentFileName() string {
	return strings.Join(gen.typeNames, nameSeparator)
}

type ElementTemplateFunctions[T any] interface {
	HasConformance(T) bool
	IsEnum(T) bool
	DeclKeywords(T) string
	DeclTypeTitle(T) string
	GenInitializer(T) bool
	Enums(declarations []T) []T
	StructsAndResources([]T) []T
	Events([]T) []T
}

type ASTDeclarationTemplateFunctions struct{}

var _ ElementTemplateFunctions[ast.Declaration] = ASTDeclarationTemplateFunctions{}

func (ASTDeclarationTemplateFunctions) HasConformance(declaration ast.Declaration) bool {
	switch declaration.DeclarationKind() {
	case common.DeclarationKindStructure,
		common.DeclarationKindResource,
		common.DeclarationKindContract,
		common.DeclarationKindEnum:
		return true
	default:
		return false
	}
}

func (ASTDeclarationTemplateFunctions) IsEnum(declaration ast.Declaration) bool {
	return declaration.DeclarationKind() == common.DeclarationKindEnum
}

func (ASTDeclarationTemplateFunctions) DeclKeywords(declaration ast.Declaration) string {
	var parts []string

	accessKeyword := declaration.DeclarationAccess().Keyword()
	if len(accessKeyword) > 0 {
		parts = append(parts, accessKeyword)
	}

	var kindKeyword string
	kind := declaration.DeclarationKind()
	if kind == common.DeclarationKindField {
		kindKeyword = declaration.(*ast.FieldDeclaration).VariableKind.Keyword()
	} else {
		kindKeyword = kind.Keywords()
	}
	if len(kindKeyword) > 0 {
		parts = append(parts, kindKeyword)
	}

	return strings.Join(parts, " ")
}

func (ASTDeclarationTemplateFunctions) DeclTypeTitle(declaration ast.Declaration) string {
	return strings.Title(declaration.DeclarationKind().Keywords())
}

func (ASTDeclarationTemplateFunctions) GenInitializer(declaration ast.Declaration) bool {
	switch declaration.DeclarationKind() {
	case common.DeclarationKindStructure,
		common.DeclarationKindResource:
		return true
	default:
		return false
	}
}

func (ASTDeclarationTemplateFunctions) Enums(declarations []ast.Declaration) []ast.Declaration {
	var enums []ast.Declaration

	for _, decl := range declarations {
		if decl.DeclarationKind() == common.DeclarationKindEnum {
			enums = append(enums, decl)
		}
	}

	return enums
}

func (ASTDeclarationTemplateFunctions) StructsAndResources(declarations []ast.Declaration) []ast.Declaration {
	var structsAndResources []ast.Declaration

	for _, declaration := range declarations {
		switch declaration.DeclarationKind() {
		case common.DeclarationKindStructure,
			common.DeclarationKindResource:
			structsAndResources = append(structsAndResources, declaration)
		}
	}

	return structsAndResources
}

func (ASTDeclarationTemplateFunctions) Events(declarations []ast.Declaration) []ast.Declaration {
	var eventDeclarations []ast.Declaration
	for _, decl := range declarations {
		if decl.DeclarationKind() == common.DeclarationKindEvent {
			eventDeclarations = append(eventDeclarations, decl)
		}
	}
	return eventDeclarations
}

func newTemplateFunctions[T any](
	elementFunctions ElementTemplateFunctions[T],
) template.FuncMap {
	return template.FuncMap{
		"hasConformance":      elementFunctions.HasConformance,
		"isEnum":              elementFunctions.IsEnum,
		"declKeywords":        elementFunctions.DeclKeywords,
		"declTypeTitle":       elementFunctions.DeclTypeTitle,
		"genInitializer":      elementFunctions.GenInitializer,
		"enums":               elementFunctions.Enums,
		"structsAndResources": elementFunctions.StructsAndResources,
		"events":              elementFunctions.Events,
		"formatDoc":           formatDoc,
		"formatFuncDoc":       formatFuncDoc,
	}
}

func formatDoc(docString string) string {
	var builder strings.Builder

	// Trim leading and trailing empty lines
	docString = strings.TrimSpace(docString)

	lines := strings.Split(docString, newline)

	for i, line := range lines {
		formattedLine := strings.TrimSpace(line)
		if i > 0 {
			builder.WriteString(newline)
		}
		builder.WriteString(formattedLine)
	}

	return builder.String()
}

func formatFuncDoc(docString string, genReturnType bool) string {
	var builder strings.Builder

	var params []string
	var isPrevLineEmpty bool
	var docLines int
	var returnDoc string

	// Trim leading and trailing empty lines
	docString = strings.TrimSpace(docString)

	lines := strings.Split(docString, newline)

	for _, line := range lines {
		formattedLine := strings.TrimSpace(line)

		if strings.HasPrefix(formattedLine, paramPrefix) {
			paramInfo := strings.TrimPrefix(formattedLine, paramPrefix)
			colonIndex := strings.IndexByte(paramInfo, ':')

			// If colon isn't there, cannot determine the param name.
			// Hence treat as a normal doc line.
			if colonIndex >= 0 {
				paramName := strings.TrimSpace(paramInfo[0:colonIndex])

				// If param name is empty, treat as a normal doc line.
				if len(paramName) > 0 {
					paramDoc := strings.TrimSpace(paramInfo[colonIndex+1:])

					var formattedParam string
					if len(paramDoc) > 0 {
						formattedParam = fmt.Sprintf("  - %s : _%s_", paramName, paramDoc)
					} else {
						formattedParam = fmt.Sprintf("  - %s", paramName)
					}

					params = append(params, formattedParam)
					continue
				}
			}
		} else if genReturnType && strings.HasPrefix(formattedLine, returnPrefix) {
			returnDoc = formattedLine
			continue
		}

		// Ignore the line if its a consecutive blank line.
		isLineEmpty := len(formattedLine) == 0
		if isPrevLineEmpty && isLineEmpty {
			continue
		}

		if docLines > 0 {
			builder.WriteString(newline)
		}

		builder.WriteString(formattedLine)
		isPrevLineEmpty = isLineEmpty
		docLines++
	}

	// Print the parameters
	if len(params) > 0 {
		if !isPrevLineEmpty {
			builder.WriteString(newline)
		}

		builder.WriteString(newline)
		builder.WriteString("Parameters:")

		for _, param := range params {
			builder.WriteString(newline)
			builder.WriteString(param)
		}

		isPrevLineEmpty = false
	}

	// Print the return type info
	if len(returnDoc) > 0 {
		if !isPrevLineEmpty {
			builder.WriteString(newline)
		}

		builder.WriteString(newline)

		returnInfo := strings.TrimPrefix(returnDoc, returnPrefix)
		returnInfo = strings.TrimSpace(returnInfo)
		builder.WriteString(fmt.Sprintf("Returns: %s", returnInfo))
	}

	return builder.String()
}
