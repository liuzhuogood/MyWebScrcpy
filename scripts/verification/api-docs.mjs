import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import vm from 'node:vm'

const root = new URL('../../', import.meta.url)
const methods = new Set(['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace'])
const spec = JSON.parse(await readFile(new URL('web/openapi.json', root), 'utf8'))

// Execute each page's actual renderer, including fetch, grouping, and filtering.
async function render(page, schema) {
  const html = await readFile(new URL(`web/${page}`, root), 'utf8')
  const script = html.match(/<script>([\s\S]*?)<\/script>/)[1]
  const list = { innerHTML: '', textContent: '', addEventListener() {} }
  const filter = {}
  const context = vm.createContext({
    URLSearchParams,
    window: { location: { search: '' } },
    document: { querySelector: selector => ({ '#list': list, '#filter': filter })[selector] },
    fetch: async () => ({ json: async () => schema }),
    console,
  })
  vm.runInContext(script, context, { filename: page })
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(list.textContent, '', `${page}: rendering failed`)
  return { list, filter }
}

function cards(html) {
  return [...html.matchAll(/<details class="endpoint">([\s\S]*?)<\/details>/g)].map(match => match[1])
}

const fixture = {
  components: { parameters: {
    shared: { name: 'id', in: 'path', required: true, description: 'shared-id', schema: { type: 'string' } },
    overridden: { name: 'id', in: 'path', required: true, description: 'operation-id', schema: { type: 'integer' } },
  } },
  paths: {
    '/fixture/{id}': {
      parameters: [{ $ref: '#/components/parameters/shared' }, { name: 'id', in: 'query', description: 'query-id', schema: { type: 'string' } }],
      summary: 'path-metadata', description: 'path-description', servers: [], 'x-custom': {},
      get: { tags: ['Fixture'], responses: { 200: { description: 'response-ok' } } },
      post: {
        tags: ['Fixture'], parameters: [{ $ref: '#/components/parameters/overridden' }],
        requestBody: { content: { 'application/json': { schema: { type: 'object', properties: { payload: { type: 'string' } } } } } },
        responses: { 201: { description: 'created-ok' } },
      },
    },
    '/plain': { get: { responses: { 200: { description: 'plain-ok' } } } },
  },
}

for (const page of ['api-docs.html', 'api-docs-automation.html']) {
  const actual = await render(page, spec)
  const rendered = cards(actual.list.innerHTML)
  const expected = Object.values(spec.paths).flatMap(item => Object.entries(item)
    .filter(([method, op]) => methods.has(method) && op.tags?.[0] !== '脚本'))
  assert.equal(rendered.length, expected.length, `${page}: operation count`)
  assert(!actual.list.innerHTML.includes('PARAMETERS'), `${page}: metadata rendered as operation`)
  for (const [path, item] of Object.entries(spec.paths)) {
    for (const [method, operation] of Object.entries(item)) {
      if (!methods.has(method) || operation.tags?.[0] === '脚本') continue
      const card = rendered.find(card => card.includes(`>${method.toUpperCase()}</span>`) && card.includes(`>${path}</span>`))
      assert(card, `${page}: missing ${method} ${path}`)
      for (const parameter of item.parameters ?? []) {
        const p = parameter.$ref ? spec.components.parameters[parameter.$ref.split('/').at(-1)] : parameter
        assert(card.includes(`<code>${p.name}</code>`), `${page}: missing shared parameter ${p.name} on ${method} ${path}`)
      }
      assert(card.includes('响应状态 (Responses)'), `${page}: missing responses on ${method} ${path}`)
    }
  }

  const { list, filter } = await render(page, fixture)
  const [getCard, postCard] = cards(list.innerHTML)
  assert.equal(cards(list.innerHTML).length, 3, `${page}: metadata filtering`)
  assert(getCard.includes('shared-id') && getCard.includes('query-id'), `${page}: inherited parameters and distinct locations`)
  assert(postCard.includes('operation-id') && postCard.includes('query-id'), `${page}: override and inheritance`)
  assert(!postCard.includes('shared-id'), `${page}: duplicate overridden parameter`)
  assert(postCard.includes('payload') && postCard.includes('created-ok'), `${page}: request/response preserved`)
  filter.oninput({ target: { value: 'post' } })
  assert.equal(cards(list.innerHTML).length, 1, `${page}: method filter`)
  filter.oninput({ target: { value: 'missing-endpoint' } })
  assert(list.innerHTML.includes('未找到匹配的接口'), `${page}: no-result filter`)
  console.log(`${page}: ${rendered.length} real operations; shared parameters, refs, overrides, bodies, responses, filtering passed`)
}
