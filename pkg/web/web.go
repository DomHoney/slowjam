/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package web is for generating HTML visualizations of stack logs
package web

import (
	"fmt"
	"image/color"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/google/slowjam/pkg/stackparse"
	"github.com/google/slowjam/third_party/colornames"
	"github.com/maruel/panicparse/v2/stack"
)

var ganttTemplate = `
<html>
  <head>
    <script type="text/javascript" src="https://www.gstatic.com/charts/loader.js"></script>
    <script type="text/javascript">
      google.charts.load('current', {'packages':['timeline']});

      google.charts.setOnLoadCallback(drawTimeline);
      function drawTimeline() {
        var container = document.getElementById('timeline');
        var chart = new google.visualization.Timeline(container);
        var dataTable = new google.visualization.DataTable();
        dataTable.addColumn({ type: 'string', id: 'Layer' });
        dataTable.addColumn({ type: 'string', id: 'Function' });
        dataTable.addColumn({ type: 'string', id: 'style', role: 'style' });
        dataTable.addColumn({ type: 'date', id: 'Start' });
        dataTable.addColumn({ type: 'date', id: 'End' });

        dataTable.addRows([
          {{ range $g := .TL.Goroutines }}
            {{ range $index, $layer := .Layers}}
              {{ range $layer.Calls }}
                [ '{{ $g.ID }}: {{ $g.Signature | Creator }}', '{{ .Name }}', '{{ Color .Package $index }}',  new Date({{ .StartDelta | Milliseconds }}), new Date({{ .EndDelta | Milliseconds }}) ],
              {{ end }}
            {{ end }}
          {{ end }}
        ]);
        var options = {
          avoidOverlappingGridLines: false,
        };
        chart.draw(dataTable, options);
      }
    </script>
  </head>
  <body>
    <h1>SlowJam for {{ .Duration}} ({{ .TL.Samples }} samples) - <a href="/">full</a> | <a href="/simple">simple</a></h1>
    <div id="timeline" style="width: 3200px; height: 1024px;"></div>
  </body>
</html>
`

// Render renders an HTML page representing a timeline.
func Render(w io.Writer, tl *stackparse.Timeline) error {
	updateColorMap(tl, colorMap)

	fmap := template.FuncMap{
		"Milliseconds": milliseconds,
		"Creator":      creator,
		"Height":       height,
		"Color":        callColor,
	}

	t, err := template.New("timeline").Funcs(fmap).Parse(ganttTemplate)
	if err != nil {
		return fmt.Errorf("template: %w", err)
	}

	rc := struct {
		Duration time.Duration
		TL       *stackparse.Timeline
	}{
		Duration: tl.End.Sub(tl.Start),
		TL:       tl,
	}

	err = t.ExecuteTemplate(w, "timeline", rc)
	if err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	return nil
}

func milliseconds(d time.Duration) string {
	return fmt.Sprintf("%d", d.Milliseconds())
}

func creator(s *stack.Signature) string {
	c := s.CreatedBy.Calls[0].Func.Name
	if c == "" {
		c = "main"
	}

	return c
}

func height(ls []*stackparse.Layer) string {
	return fmt.Sprintf("%d", 100+(35*len(ls)))
}

func updateColorMap(tl *stackparse.Timeline, cm map[string]color.RGBA) {
	chosen := map[string]bool{}

	for _, g := range tl.Goroutines {
		for _, l := range g.Layers {
			for _, c := range l.Calls {
				_, ok := cm[c.Package]
				if ok {
					continue
				}

				// gimmick: If a package is named after a color, use it
				for name, value := range colornames.Map {
					if strings.Contains(name, "white") {
						continue
					}

					if name[0] != c.Package[0] {
						continue
					}

					if !chosen[name] {
						chosen[name] = true
						cm[c.Package] = value

						break
					}
				}

				_, ok = cm[c.Package]
				if ok {
					continue
				}

				// Giveup
				for name, value := range colornames.Map {
					if strings.Contains(name, "white") {
						continue
					}

					if !chosen[name] {
						chosen[name] = true
						cm[c.Package] = value

						break
					}
				}
			}
		}
	}
}

func callColor(pkg string, level int) string {
	c := colorMap[pkg]
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}
