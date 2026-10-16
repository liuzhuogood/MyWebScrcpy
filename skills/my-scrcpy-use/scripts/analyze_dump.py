#!/usr/bin/env python3
"""Analyze a confirmed /api/ui/xml JSON response; never operates a device."""

import argparse
from collections import Counter
import json
from pathlib import Path
import re
import sys
import xml.etree.ElementTree as ET


def analyze(response: dict, expected_serial: str) -> dict:
    if not isinstance(response, dict):
        raise ValueError('响应必须是 JSON 对象')
    for key in ('serial', 'captured_at', 'xml'):
        if key not in response or not isinstance(response[key], str) or not response[key]:
            raise ValueError(f'响应缺少有效的 {key} 字段；请核对当前接口定义')
    if response['serial'] != expected_serial:
        raise ValueError('响应 serial 与预期设备不符')
    xml = response['xml']
    if re.search(r'<!\s*(DOCTYPE|ENTITY)\b', xml, re.IGNORECASE):
        raise ValueError('UI dump 不应包含 DTD 或实体声明')
    root = ET.fromstring(xml)
    if root.tag != 'hierarchy':
        raise ValueError('XML 根节点不是 hierarchy；请核对 dump 格式')
    warnings = []
    display = None
    if 'display_size' in response:
        size = response['display_size']
        if not isinstance(size, dict):
            raise ValueError('display_size 必须是对象')
        if 'width' in size and 'height' in size:
            if any(type(size[key]) is not int or size[key] <= 0 for key in ('width', 'height')):
                raise ValueError('display_size 的 width、height 必须是正整数')
            display = {'width': size['width'], 'height': size['height']}
    if display is None:
        warnings.append('屏幕坐标基准缺失，不生成中心点，不从节点范围推算分辨率。')
    nodes = []

    def visit(element, parent_id, ancestors, path):
        if element.tag != 'node':
            raise ValueError('hierarchy 中存在非 node 元素；请核对 dump 格式')
        node_id = len(nodes)
        attributes = dict(element.attrib)
        if attributes.get('password') == 'true' and 'text' in attributes:
            attributes['text'] = '[redacted]'
        node = {'node_id': node_id, 'parent_id': parent_id, 'ancestor_ids': list(ancestors),
                'tree_path': path, 'attributes': attributes}
        nodes.append(node)
        for position, child in enumerate(element, 1):
            visit(child, node_id, [*ancestors, node_id], f'{path}/node[{position}]')

    for position, element in enumerate(root, 1):
        visit(element, None, [], f'/hierarchy/node[{position}]')
    counts = Counter()
    for node in nodes:
        attrs = node['attributes']
        for field in ('resource-id', 'content-desc', 'text'):
            if field in attrs and attrs[field] and not (field == 'text' and attrs.get('password') == 'true'):
                counts[(None, field, attrs[field])] += 1
                if 'package' in attrs and attrs['package']:
                    counts[(attrs['package'], field, attrs[field])] += 1
    for node in nodes:
        attrs = node['attributes']
        selectors = []
        for field in ('resource-id', 'content-desc', 'text'):
            if field not in attrs or not attrs[field] or (field == 'text' and attrs.get('password') == 'true'):
                continue
            conditions = {field: attrs[field]}
            scope = None
            if 'package' in attrs and attrs['package']:
                conditions['package'] = attrs['package']
                scope = attrs['package']
            selectors.append({'conditions': conditions,
                              'matches_in_snapshot': counts[(scope, field, attrs[field])]})
        node['selector_candidates'] = selectors
        node['bounds'], node['center_candidate'], node['bounds_issue'] = inspect_bounds(attrs, display)
        node['declared_actions'] = [action for action, field in (
            ('click', 'clickable'), ('long_click', 'long-clickable'), ('scroll', 'scrollable'))
            if attrs.get('enabled') == 'true' and attrs.get(field) == 'true']
        click_target = None
        for target_id in [node['node_id'], *reversed(node['ancestor_ids'])]:
            target = nodes[target_id]['attributes']
            if target.get('enabled') != 'true':
                break
            if target.get('clickable') == 'true':
                click_target = target_id
                break
        node['click_target_id'] = click_target
    packages = sorted({node['attributes']['package'] for node in nodes
                       if 'package' in node['attributes'] and node['attributes']['package']})
    summary = {'node_count': len(nodes), 'packages': packages,
               'text_nodes': sum(bool(node['attributes'].get('text'))
                                 and node['attributes'].get('password') != 'true' for node in nodes),
               'description_nodes': sum(bool(node['attributes'].get('content-desc')) for node in nodes)}
    for action in ('click', 'long_click', 'scroll'):
        summary[f'{action}_marked_nodes'] = sum(action in node['declared_actions'] for node in nodes)
    if not nodes:
        warnings.append('当前 XML 没有节点，无法提供控件定位依据。')
    if packages == ['com.android.systemui']:
        warnings.append('只观察到 System UI 节点，无法确认应用业务页面，也不能据此断言锁屏。')
    if nodes and not summary['text_nodes'] and not summary['description_nodes']:
        warnings.append('没有非空文字或内容描述，无法仅凭 dump 确认页面含义。')
    if nodes and not any(node['declared_actions'] for node in nodes):
        warnings.append('没有明确启用且标记为可点击、长按或滚动的节点。')
    return {'serial': response['serial'], 'captured_at': response['captured_at'],
            'display_size': display, 'hierarchy_attributes': dict(root.attrib),
            'summary': summary, 'warnings': warnings, 'nodes': nodes}


def inspect_bounds(attributes: dict, display: dict | None) -> tuple:
    if 'bounds' not in attributes:
        return None, None, 'missing_bounds'
    match = re.fullmatch(r'\[(-?\d+),(-?\d+)\]\[(-?\d+),(-?\d+)\]', attributes['bounds'])
    if match is None:
        return None, None, 'invalid_bounds'
    x1, y1, x2, y2 = rectangle = list(map(int, match.groups()))
    if x2 <= x1 or y2 <= y1:
        return rectangle, None, 'empty_or_reversed_bounds'
    if display is None:
        return rectangle, None, 'missing_display_size'
    if not (0 <= x1 < x2 <= display['width'] and 0 <= y1 < y2 <= display['height']):
        return rectangle, None, 'outside_display_size'
    return rectangle, [(x1 + x2) // 2, (y1 + y2) // 2], None


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('response', type=Path, help='接口原始 JSON 响应文件')
    parser.add_argument('--expected-serial', required=True)
    parser.add_argument('--output', type=Path, help='分析结果 JSON；默认标准输出')
    parser.add_argument('--xml-output', type=Path, help='另存原始 XML，可能含敏感页面数据')
    parser.add_argument('--page-info-before', type=Path, help='抓取 XML 前的 page-info 响应')
    parser.add_argument('--page-info-after', type=Path, help='抓取 XML 后的 page-info 响应')
    options = parser.parse_args()
    if bool(options.page_info_before) != bool(options.page_info_after):
        parser.error('页面信息前后快照必须一起提供')
    try:
        response = json.loads(options.response.read_text(encoding='utf-8'))
        report = analyze(response, options.expected_serial)
        if options.page_info_before:
            from page_context import correlate
            before = json.loads(options.page_info_before.read_text(encoding='utf-8'))
            after = json.loads(options.page_info_after.read_text(encoding='utf-8'))
            report['page_context'] = correlate(report, before, after)
        if options.xml_output:
            options.xml_output.write_text(response['xml'], encoding='utf-8')
        text = json.dumps(report, ensure_ascii=False, indent=2) + '\n'
        if options.output:
            options.output.write_text(text, encoding='utf-8')
        else:
            sys.stdout.write(text)
        print(f"[ui-dump] 已分析 {report['summary']['node_count']} 个节点", file=sys.stderr)
        return 0
    except (ValueError, OSError, ET.ParseError) as error:
        print(f'[ui-dump] 分析失败：{error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
