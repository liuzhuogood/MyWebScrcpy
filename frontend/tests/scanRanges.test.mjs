import test from 'node:test'
import assert from 'node:assert/strict'
import { formatScanRange } from '../src/utils/scanRanges.ts'
test('segmented IPv4 validates bounds, order, size and local addresses',()=>{
 assert.equal(formatScanRange(['010','0','0','1'],['10','0','0','2']),'10.0.0.1-10.0.0.2')
 for(const [a,b] of [['10.0.0.256','10.0.0.257'],['10.0..1','10.0.0.2'],['10.0.0.4','10.0.0.3'],['10.0.0.1','10.0.1.1'],['8.8.8.8','8.8.8.9']]) assert.throws(()=>formatScanRange(a.split('.'),b.split('.')))
})
