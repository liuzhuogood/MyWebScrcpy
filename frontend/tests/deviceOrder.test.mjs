import test from 'node:test'
import assert from 'node:assert/strict'
import { orderDevices, reorderVisibleDevices, buildNumberMap } from '../src/utils/deviceOrder.ts'

test('one full-list number map survives offline gaps, reordering and visibility', () => {
 const devices = [{serial:'a',state:'device'},{serial:'b',state:'offline'},{serial:'c',state:'device'}]
 assert.equal(buildNumberMap(devices).get('c'), '03')
 const order = reorderVisibleDevices(devices.map(d=>d.serial), ['c','a'])
 assert.deepEqual(order, ['c','b','a'])
 const sorted = orderDevices(devices,order)
 assert.equal(buildNumberMap(sorted).get('c'),'01')
 const selected = new Set(['c'])
 assert.deepEqual(sorted.filter(d=>selected.has(d.serial)).map(d=>d.serial),['c'])
 assert.equal(selected.has('b'),false)
 assert.deepEqual(orderDevices([...devices].reverse(),order), sorted)
 assert.deepEqual(orderDevices([...devices,{serial:'d'}],order).map(d=>d.serial),['c','b','a','d'])
})
