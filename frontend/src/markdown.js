// 轻量 Markdown 渲染：先整体转义 HTML，再生成受控标签，
// 所以正文里写什么都无法注入脚本。支持标题、粗体/斜体/删除线、
// 行内代码、代码块、引用、有序/无序列表、分割线、链接。

const ENT = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }
const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ENT[c])

function safeUrl(url) {
  const u = String(url).trim()
  if (!/^(https?:\/\/|mailto:)/i.test(u)) return ''
  return esc(u)
}

function inline(raw) {
  let t = esc(raw)

  // 行内代码先抽出来，避免里面的 * _ 被当成强调
  const spans = []
  t = t.replace(/`([^`\n]+)`/g, (_, code) => {
    spans.push(`<code>${code}</code>`)
    return `\u0001${spans.length - 1}\u0001`
  })

  t = t.replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>')
  t = t.replace(/__([^_\n]+)__/g, '<strong>$1</strong>')
  t = t.replace(/(^|[^\w*])\*([^*\n]+)\*/g, '$1<em>$2</em>')
  t = t.replace(/(^|[^\w_])_([^_\n]+)_/g, '$1<em>$2</em>')
  t = t.replace(/~~([^~\n]+)~~/g, '<del>$1</del>')

  t = t.replace(/\[([^\]\n]+)\]\(([^)\s]+)\)/g, (m, text, url) => {
    const u = safeUrl(url)
    return u ? `<a href="${u}" target="_blank" rel="noreferrer noopener">${text}</a>` : m
  })

  t = t.replace(/\n/g, '<br>')
  t = t.replace(/\u0001(\d+)\u0001/g, (_, n) => spans[+n])
  return t
}

const RE_HR = /^ {0,3}([-*_])(?:\s*\1){2,}\s*$/
const RE_UL = /^ {0,3}[-*+]\s+(.*)$/
const RE_OL = /^ {0,3}(\d{1,9})[.)]\s+(.*)$/
const RE_QUOTE = /^ {0,3}>/
const RE_HEAD = /^ {0,3}(#{1,6})\s+(.*?)\s*#*\s*$/

export function renderMarkdown(src) {
  if (!src) return ''
  let text = String(src).replace(/\r\n?/g, '\n')

  // 围栏代码块：整段原样保留
  const blocks = []
  text = text.replace(/^```[^\n]*\n([\s\S]*?)^```[ \t]*$/gm, (_, body) => {
    blocks.push(`<pre><code>${esc(body.replace(/\n+$/, ''))}</code></pre>`)
    return `\u0000B${blocks.length - 1}\u0000`
  })

  const lines = text.split('\n')
  const out = []
  let i = 0
  let para = []

  const flush = () => {
    if (para.length) {
      out.push(`<p>${inline(para.join('\n'))}</p>`)
      para = []
    }
  }

  while (i < lines.length) {
    const line = lines[i]

    if (!line.trim()) {
      flush()
      i++
      continue
    }

    const token = line.match(/^\u0000B(\d+)\u0000$/)
    if (token) {
      flush()
      out.push(blocks[+token[1]])
      i++
      continue
    }

    if (RE_HR.test(line)) {
      flush()
      out.push('<hr>')
      i++
      continue
    }

    const head = line.match(RE_HEAD)
    if (head) {
      flush()
      const level = head[1].length
      out.push(`<h${level}>${inline(head[2])}</h${level}>`)
      i++
      continue
    }

    if (RE_QUOTE.test(line)) {
      flush()
      const buf = []
      while (i < lines.length && RE_QUOTE.test(lines[i])) {
        buf.push(lines[i].replace(/^ {0,3}>\s?/, ''))
        i++
      }
      out.push(`<blockquote>${renderMarkdown(buf.join('\n'))}</blockquote>`)
      continue
    }

    // 列表：连续的同类标记合成一个 <ul>/<ol>，缩进的续行并入上一项
    const ul = line.match(RE_UL)
    const ol = line.match(RE_OL)
    if (ul || ol) {
      flush()
      const ordered = !!ol
      const items = []
      while (i < lines.length) {
        const cur = lines[i]
        const m = ordered ? cur.match(RE_OL) : cur.match(RE_UL)
        if (m) {
          items.push(ordered ? m[2] : m[1])
          i++
          continue
        }
        if (items.length && /^\s{2,}\S/.test(cur)) {
          items[items.length - 1] += '\n' + cur.trim()
          i++
          continue
        }
        break
      }
      const tag = ordered ? 'ol' : 'ul'
      const li = items.map((x) => `<li>${inline(x)}</li>`).join('')
      out.push(`<${tag}>${li}</${tag}>`)
      continue
    }

    para.push(line)
    i++
  }

  flush()
  return out.join('\n')
}

export default renderMarkdown
