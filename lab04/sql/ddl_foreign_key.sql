FOREIGN KEY ({{quote .Col}}) REFERENCES {{quote .ForeignTable}} ({{quote .ForeignCol}}){{if .OnDelete}} ON DELETE {{.OnDelete}}{{end}}{{if .OnUpdate}} ON UPDATE {{.OnUpdate}}{{end}}
