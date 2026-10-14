import base64
import io
import json
import sys
import unittest
import zipfile
from dataclasses import replace
from unittest.mock import patch

import cv2
import httpx

from mywebscrcpy import Client, HTTPError, SDKError, TemplateDisabled, UnsupportedTemplateMethod
from mywebscrcpy.contracts import Deadline
from mywebscrcpy.dependencies import vision_dependencies
from mywebscrcpy.errors import MissingDependency
from mywebscrcpy.templates import TemplateCache, template_from_wire
from test_vision import assets, template_wire


class AssetsTests(unittest.TestCase):
    def test_crud_images_and_zip_exact_contract(self):
        pattern,_,_,_=assets(); _,png=cv2.imencode('.png',pattern)
        png=png.tobytes()
        zip_buffer=io.BytesIO()
        with zipfile.ZipFile(zip_buffer,'w') as archive: archive.writestr('target.png',png)
        wire=template_wire(); requests=[]
        def handler(request):
            requests.append(request)
            path=request.url.path
            if path.endswith('/image'): return httpx.Response(200,content=png,headers={'content-type':'image/png'})
            if path.endswith('/export'): return httpx.Response(200,content=zip_buffer.getvalue(),headers={'content-type':'application/zip'})
            if path.endswith('/import'): return httpx.Response(200,json={'success':True,'imported':1,'templates':[wire]})
            if path == '/api/devices': return httpx.Response(200,json=[{'serial':'phone','state':'device','source':'manual','auto_reconnect':True}])
            if request.method == 'DELETE': return httpx.Response(200,json={'ok':True,'id':'target'})
            if path == '/api/templates' and request.method=='GET': return httpx.Response(200,json={'templates':[wire],'total':1})
            return httpx.Response(200,json=wire)
        with Client('http://fixture') as client:
            client._http.close(); client._http=httpx.Client(base_url='http://fixture',transport=httpx.MockTransport(handler))
            self.assertEqual(client.devices.list()[0].alias,'')
            self.assertEqual(client.templates.list(serial='phone')[0].id,'target')
            client.templates.create('目标',png,serial='phone',threshold=.95)
            body=json.loads(requests[-1].content)
            self.assertEqual(base64.b64decode(body['image_base64']),png)
            client.templates.update('target',enabled=False)
            self.assertEqual(json.loads(requests[-1].content),{'enabled':False})
            self.assertEqual(client.templates.image('target'),png)
            client.templates.add_image('target',png)
            self.assertIn(b'name="image"',requests[-1].content)
            with zipfile.ZipFile(io.BytesIO(client.templates.export_zip())) as archive:
                self.assertEqual(archive.namelist(),['target.png'])
            self.assertEqual(client.templates.import_zip(zip_buffer.getvalue()).imported,1)
            self.assertIn(b'name="file"',requests[-1].content)
            client.templates.delete('target')

    def test_scope_method_disabled_and_refresh_failure_do_not_fallback(self):
        wire=template_wire(); pattern,_,_,_=assets(); _,png=cv2.imencode('.png',pattern)
        failure=False; image_requests=[]
        def handler(request):
            if failure: return httpx.Response(503,json={'error':'unavailable','message':'停止刷新'})
            if request.url.path.endswith('/image'):
                image_requests.append(request)
                return httpx.Response(200,content=png.tobytes(),headers={'content-type':'image/png'})
            return httpx.Response(200,json={'templates':[wire],'total':1})
        with Client('http://fixture') as client:
            client._http.close(); client._http=httpx.Client(base_url='http://fixture',transport=httpx.MockTransport(handler))
            cache=TemplateCache(client,'phone'); template=template_from_wire(wire)
            for value,error in [(replace(template,serial='other'),SDKError),(replace(template,enabled=False),TemplateDisabled),(replace(template,method='TM_SQDIFF_NORMED'),UnsupportedTemplateMethod)]:
                with self.assertRaises(error): cache.load(value,Deadline(1))
            self.assertFalse(image_requests)
            cache.refresh(Deadline(1)); cache.refresh(Deadline(1))
            self.assertEqual(len(image_requests),1)
            failure=True
            with self.assertRaises(HTTPError): cache.refresh(Deadline(1))

    def test_missing_vision_has_install_hint(self):
        with patch('mywebscrcpy.dependencies.import_module',side_effect=ImportError('av')):
            with self.assertRaisesRegex(MissingDependency,r'\[vision\]'): vision_dependencies()
