"""Template HTTP assets; bytes stay bytes until vision is explicitly requested."""
import base64
import logging
from urllib.parse import quote

from .contracts import Deadline, field
from .errors import ProtocolError, SDKError, TemplateDisabled, UnsupportedTemplateMethod
from .models import DeviceInfo, ImportResult, Template

logger = logging.getLogger(__name__)
CREATE_FIELDS = {'id', 'threshold', 'method', 'grayscale', 'scales', 'enabled', 'scene_width', 'scene_height'}
UPDATE_FIELDS = CREATE_FIELDS - {'id'} | {'name', 'serial'}


def template_from_wire(value):
    required = {'id': str, 'name': str, 'serial': str, 'threshold': float, 'method': str,
                'grayscale': bool, 'enabled': bool, 'width': int, 'height': int,
                'image_count': int, 'created_at': str, 'updated_at': str}
    values = {name: field(value, name, kind) for name, kind in required.items()}
    scales = field(value, 'scales', list, [1.0])
    if not scales or any(type(v) not in {int, float} or not 0 < v < float('inf') for v in scales):
        raise ProtocolError('scales 必须包含有限正数')
    if not values['id'] or not values['name'] or not 0 < values['threshold'] <= 1 or any(values[n] <= 0 for n in ('width', 'height', 'image_count')):
        raise ProtocolError('模板字段范围不合法')
    values.update(scales=tuple(scales), scene_width=field(value, 'scene_width', int, 0), scene_height=field(value, 'scene_height', int, 0))
    return Template(**values)


def asset_path(template_id):
    if not isinstance(template_id, str) or not template_id.strip() or template_id in {'.', '..'} or '/' in template_id or '\\' in template_id:
        raise ValueError('template_id 必须是明确的 ID')
    return '/api/templates/' + quote(template_id, safe='')


class Templates:
    def __init__(self, client):
        self._client = client

    def list(self, *, serial='all', timeout=10):
        value = self._client._json('GET', '/api/templates', params={'serial': serial}, timeout=timeout)
        items = field(value, 'templates', list)
        if field(value, 'total', int) != len(items):
            raise ProtocolError('模板 total 与列表不一致')
        return [template_from_wire(v) for v in items]

    def get(self, template_id, *, serial=None, timeout=10):
        result = template_from_wire(self._client._json('GET', asset_path(template_id), params={'serial': serial} if serial is not None else None, timeout=timeout))
        if result.id != template_id:
            raise ProtocolError('模板返回 ID 不一致')
        return result

    def image(self, template_id, *, serial=None, timeout=10):
        response = self._client._response('GET', asset_path(template_id) + '/image', params={'serial': serial} if serial is not None else None, timeout=timeout)
        if response.headers.get('content-type', '').split(';')[0] != 'image/png':
            raise ProtocolError('主图片必须是 image/png')
        return response.content

    def create(self, name, image: bytes, *, serial='global', timeout=10, **fields):
        if set(fields) - CREATE_FIELDS:
            raise ValueError('未知模板创建字段')
        return template_from_wire(self._client._json('POST', '/api/templates', body=dict(fields, name=name, serial=serial, image_base64=base64.b64encode(image).decode('ascii')), timeout=timeout))

    def update(self, template_id, *, image: bytes | None = None, timeout=10, **fields):
        if set(fields) - UPDATE_FIELDS:
            raise ValueError('未知模板更新字段')
        if image is not None:
            fields['image_base64'] = base64.b64encode(image).decode('ascii')
        return template_from_wire(self._client._json('PUT', asset_path(template_id), body=fields, timeout=timeout))

    def delete(self, template_id, *, timeout=10):
        value = self._client._json('DELETE', asset_path(template_id), timeout=timeout)
        if field(value, 'ok', bool) is not True or field(value, 'id', str) != template_id:
            raise ProtocolError('删除返回不合法')

    def add_image(self, template_id, image: bytes, *, timeout=10):
        return template_from_wire(self._client._json('POST', asset_path(template_id) + '/images', files={'image': ('image.png', image, 'image/png')}, timeout=timeout))

    def export_zip(self, *, serial='all', scope='all', timeout=30):
        if scope not in {'all', 'global', 'device'}:
            raise ValueError('scope 必须为 all/global/device')
        response = self._client._response('GET', '/api/templates/export', params={'serial': serial, 'scope': scope}, timeout=timeout)
        if response.headers.get('content-type', '').split(';')[0] != 'application/zip':
            raise ProtocolError('导出必须是 ZIP')
        return response.content

    def import_zip(self, data: bytes, *, serial='global', timeout=30):
        value = self._client._json('POST', '/api/templates/import', data={'serial': serial}, files={'file': ('templates.zip', data, 'application/zip')}, timeout=timeout)
        templates = tuple(template_from_wire(v) for v in field(value, 'templates', list))
        result = ImportResult(field(value, 'success', bool), field(value, 'imported', int), templates)
        if result.imported != len(templates) or not result.success:
            raise ProtocolError('ZIP 导入返回不一致')
        return result


class Devices:
    def __init__(self, client):
        self._client = client

    def list(self, *, timeout=10):
        value = self._client._json('GET', '/api/devices', timeout=timeout)
        if not isinstance(value, list):
            raise ProtocolError('设备响应必须是数组')
        return [DeviceInfo(**{name: field(v, name, kind) for name, kind in {'serial': str, 'state': str, 'source': str, 'auto_reconnect': bool}.items()},
                           **{name: field(v, name, str, '') for name in ('alias', 'product', 'model', 'device', 'transport_id')}) for v in value]


class TemplateCache:
    """Refresh failures propagate; no disk fallback and no stale actions."""
    def __init__(self, client, serial):
        self.client, self.serial, self._items = client, serial, {}

    def load(self, template, deadline):
        from .dependencies import vision_dependencies
        _, cv2, np = vision_dependencies()
        if template.serial not in {'', 'global', self.serial}:
            raise SDKError('模板属于其他设备')
        if not template.enabled:
            raise TemplateDisabled(template.id)
        if template.method != 'TM_CCOEFF_NORMED':
            raise UnsupportedTemplateMethod(template.method)
        cached = self._items.get(template.id)
        if cached is not None and cached[0] == template:
            return cached[1]
        data = self.client.templates.image(template.id, serial=self.serial, timeout=deadline.remaining())
        image = cv2.imdecode(np.frombuffer(data, dtype=np.uint8), cv2.IMREAD_COLOR)
        if image is None or image.shape[:2] != (template.height, template.width):
            raise ProtocolError('模板主图片与尺寸不一致')
        if template.image_count > 1:
            logger.warning('Only primary template image is matched: id=%s image_count=%s', template.id, template.image_count)
        self._items[template.id] = (template, image)
        return image

    def get(self, template_id, deadline):
        template = self.client.templates.get(template_id, serial=self.serial, timeout=deadline.remaining())
        return template, self.load(template, deadline)

    def refresh(self, deadline):
        logger.info('Refreshing templates: serial=%s', self.serial)
        items = self.client.templates.list(serial=self.serial, timeout=deadline.remaining())
        loaded = [(template, self.load(template, deadline)) for template in items if template.enabled]
        ids = {template.id for template in items}
        self._items = {key: value for key, value in self._items.items() if key in ids}
        return loaded
