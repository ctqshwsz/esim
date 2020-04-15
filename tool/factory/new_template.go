package factory

var newTemplate = `
{{.StructTpl.String}}

{{.TypePluralStr}}

{{.Option1}}

{{.Option2}}

{{if .WithNew}}
func New{{.StructName}}({{.Option3}}) {{.NewStructInfo.ReturnVarStr}}{

{{.NewStructInfo.StructInitStr}}

{{.Option4}}

{{.SpecFieldInitStr}}

	return {{.ReturnStr}}
}
{{end}}

{{.Option5}}

{{.Option6}}

{{.ReleaseStr}}

{{.NewPluralStr}}

{{.ReleasePluralStr}}
`