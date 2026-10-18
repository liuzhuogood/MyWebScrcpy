package devicestate

import (
	"html/template"
	"io"
	"time"
)

const htmlPageTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>设备总状态 · {{ .DeviceState.Serial }}</title>
  <style>
    :root {
      --bg: #09090b;
      --card-bg: #18181b;
      --border: #27272a;
      --text: #f4f4f5;
      --text-muted: #a1a1aa;
      --primary: #3b82f6;
      --success: #10b981;
      --warning: #f59e0b;
      --destructive: #ef4444;
    }
    @media (prefers-color-scheme: light) {
      :root {
        --bg: #f8fafc;
        --card-bg: #ffffff;
        --border: #e2e8f0;
        --text: #0f172a;
        --text-muted: #64748b;
        --primary: #2563eb;
      }
    }
    body {
      margin: 0;
      padding: 16px;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      background: var(--bg);
      color: var(--text);
      font-size: 14px;
      line-height: 1.5;
    }
    .container {
      max-width: 1100px;
      margin: 0 auto;
      display: flex;
      flex-direction: column;
      gap: 16px;
    }
    header {
      display: flex;
      flex-wrap: wrap;
      justify-content: space-between;
      align-items: center;
      gap: 12px;
      padding-bottom: 12px;
      border-bottom: 1px solid var(--border);
    }
    .title-group h1 {
      margin: 0;
      font-size: 18px;
      font-weight: 600;
    }
    .meta-text {
      font-size: 12px;
      color: var(--text-muted);
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      margin-top: 4px;
    }
    .badge {
      display: inline-block;
      padding: 2px 8px;
      border-radius: 9999px;
      font-size: 11px;
      font-weight: 500;
      background: var(--border);
    }
    .badge-success { background: #064e3b; color: #6ee7b7; }
    .badge-warning { background: #78350f; color: #fde68a; }
    .badge-destructive { background: #7f1d1d; color: #fca5a5; }
    .badge-primary { background: #1e3a8a; color: #bfdbfe; }
    .btn {
      display: inline-flex;
      align-items: center;
      padding: 6px 14px;
      border-radius: 6px;
      background: var(--primary);
      color: #fff;
      text-decoration: none;
      font-size: 13px;
      font-weight: 500;
      cursor: pointer;
      border: none;
    }
    .grid {
      display: grid;
      grid-template-columns: 1fr;
      gap: 16px;
    }
    @media (min-width: 840px) {
      .grid { grid-template-columns: 420px 1fr; }
    }
    .card {
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 16px;
      display: flex;
      flex-direction: column;
      gap: 12px;
    }
    .card h2 {
      margin: 0;
      font-size: 14px;
      font-weight: 600;
      border-bottom: 1px solid var(--border);
      padding-bottom: 8px;
    }
    .screenshot-img {
      width: 100%;
      max-height: 520px;
      object-fit: contain;
      background: #000;
      border-radius: 6px;
      border: 1px solid var(--border);
    }
    dl {
      margin: 0;
      display: grid;
      grid-template-columns: 100px 1fr;
      row-gap: 8px;
      font-size: 12px;
    }
    dt { color: var(--text-muted); }
    dd { margin: 0; word-break: break-all; }
    .decision-box {
      border: 1px solid var(--primary);
      background: rgba(59, 130, 246, 0.05);
      border-radius: 6px;
      padding: 12px;
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    .decision-item-title {
      font-size: 12px;
      color: var(--text-muted);
      font-weight: 600;
    }
    .decision-item-val {
      font-size: 13px;
      margin-top: 2px;
    }
    ul { margin: 0; padding-left: 18px; }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div class="title-group">
        <h1>设备总状态 · {{ .DeviceState.Serial }}</h1>
        <div class="meta-text">
          观测 ID: {{ .DeviceState.ObservationID }} · 采集时间: {{ formatTime .DeviceState.Timestamp }}
          {{ if .DeviceState.ContextStable }}
            <span class="badge badge-success">上下文稳定</span>
          {{ else }}
            <span class="badge badge-warning">上下文波动</span>
          {{ end }}
        </div>
      </div>
      <div>
        <a href="" class="btn">手动刷新</a>
      </div>
    </header>

    <div class="grid">
      <!-- 左列：截图与设备信息 -->
      <div class="card">
        <h2>本轮屏幕画面</h2>
        {{ if .DeviceState.Screenshot.DataBase64 }}
          <img src="{{ safeURL .DeviceState.Screenshot.DataBase64 }}" alt="设备截图" class="screenshot-img" />
          <div class="meta-text" style="text-align: right;">
            分辨率: {{ .DeviceState.Screenshot.Width }}x{{ .DeviceState.Screenshot.Height }} · {{ .DeviceState.Screenshot.SizeBytes }} 字节
          </div>
        {{ else }}
          <div class="meta-text">无截图数据</div>
        {{ end }}

        <h2 style="margin-top: 8px;">前台与设备指标</h2>
        <dl>
          <dt>应用包名</dt>
          <dd>{{ if .DeviceState.PageInfo }}{{ .DeviceState.PageInfo.PackageName }}{{ else }}未知{{ end }}</dd>
          <dt>Activity</dt>
          <dd>{{ if .DeviceState.PageInfo }}{{ .DeviceState.PageInfo.Activity }}{{ else }}未知{{ end }}</dd>
          <dt>旋转角度</dt>
          <dd>{{ if .DeviceState.PageInfo }}{{ if .DeviceState.PageInfo.Display }}{{ .DeviceState.PageInfo.Display.Rotation }}°{{ else }}0°{{ end }}{{ else }}0°{{ end }}</dd>
        </dl>
      </div>

      <!-- 右列：决策结果、OCR与模板匹配 -->
      <div style="display: flex; flex-direction: column; gap: 16px;">
        {{ if .HasDecision }}
          <div class="card">
            <h2>AI 视觉决策 (Chat Completions)</h2>
            {{ if .Decision.Available }}
              <div class="decision-box">
                <div>
                  <div class="decision-item-title">画面摘要</div>
                  <div class="decision-item-val">{{ .Decision.PageSummary }}</div>
                </div>
                <div>
                  <div class="decision-item-title">可见控件与弹层</div>
                  <div class="decision-item-val">
                    {{ if .Decision.VisibleControls }}
                      <ul>
                        {{ range .Decision.VisibleControls }}
                          <li>{{ . }}</li>
                        {{ end }}
                      </ul>
                    {{ else }}
                      无明显控件
                    {{ end }}
                  </div>
                </div>
                <div>
                  <div class="decision-item-title">任务进展判断</div>
                  <div class="decision-item-val">{{ .Decision.TaskProgress }}</div>
                </div>
                <div style="background: rgba(16, 185, 129, 0.1); padding: 8px; border-radius: 4px; border: 1px solid var(--success);">
                  <div class="decision-item-title" style="color: var(--success);">下一步建议（参考）</div>
                  <div class="decision-item-val" style="font-weight: 500;">{{ .Decision.NextStep }}</div>
                  <div class="meta-text" style="margin-top: 4px;">依据: {{ .Decision.Rationale }}</div>
                </div>
                {{ if .Decision.Uncertainties }}
                  <div>
                    <div class="decision-item-title">不确定项</div>
                    <ul class="meta-text">
                      {{ range .Decision.Uncertainties }}
                        <li>{{ . }}</li>
                      {{ end }}
                    </ul>
                  </div>
                {{ end }}
                <div class="meta-text" style="text-align: right; margin-top: 4px;">
                  模型: {{ .Decision.Model }} · 耗时: {{ .Decision.DurationMS }}ms · 绑定观测: {{ .Decision.ObservationID }}
                </div>
              </div>
            {{ else }}
              <div class="badge badge-warning" style="padding: 8px 12px; border-radius: 6px;">
                决策不可用：{{ .Decision.Error }}
              </div>
            {{ end }}
          </div>
        {{ end }}


        <!-- 模板匹配卡片 -->
        <div class="card">
          <h2>模板匹配 (全局 · 设备 · 技能) ({{ len .DeviceState.TemplateMatches }})</h2>
          {{ if .DeviceState.TemplateMatches }}
            <dl>
              {{ range .DeviceState.TemplateMatches }}
                <dt><span class="badge">{{ .Source }}</span></dt>
                <dd>{{ .TemplateName }} (置信度: {{ printf "%.2f" .Confidence }}) 坐标: [{{ .PixelX }}, {{ .PixelY }}]</dd>
              {{ end }}
            </dl>
          {{ else }}
            <div class="meta-text">本轮未命中模板</div>
          {{ end }}
        </div>

        <!-- OCR 结果卡片 -->
        <div class="card">
          <h2>OCR 文字提取 ({{ len .DeviceState.OCR.Boxes }} 项)</h2>
          {{ if .DeviceState.OCR.Error }}
            <div class="badge badge-destructive">OCR 失败: {{ .DeviceState.OCR.Error }}</div>
          {{ else if .DeviceState.OCR.Boxes }}
            <div class="meta-text" style="max-height: 160px; overflow-y: auto; background: var(--bg); padding: 8px; border-radius: 4px;">
              {{ .DeviceState.OCR.Text }}
            </div>
          {{ else }}
            <div class="meta-text">未提取到可见文字</div>
          {{ end }}
        </div>

        {{ if .DeviceState.PartialFailures }}
          <div class="card">
            <h2>部分采集异常记录</h2>
            <ul style="color: var(--destructive); font-size: 12px;">
              {{ range .DeviceState.PartialFailures }}
                <li>{{ . }}</li>
              {{ end }}
            </ul>
          </div>
        {{ end }}
      </div>
    </div>
  </div>
</body>
</html>`

type HTMLRenderData struct {
	DeviceState DeviceState
	Decision    AIDecisionResult
	HasDecision bool
}

var parsedTemplate = template.Must(template.New("device_state").Funcs(template.FuncMap{
	"formatTime": func(ms int64) string {
		if ms <= 0 {
			return "-"
		}
		return time.UnixMilli(ms).Format("2006-01-02 15:04:05")
	},
	"safeURL": func(s string) template.URL {
		return template.URL(s)
	},
}).Parse(htmlPageTemplate))

// RenderHTML 渲染简单服务端 HTML 页面
func RenderHTML(w io.Writer, state DeviceState, decision *AIDecisionResult) error {
	data := HTMLRenderData{
		DeviceState: state,
	}
	if decision != nil {
		data.Decision = *decision
		data.HasDecision = true
	}
	return parsedTemplate.Execute(w, data)
}
