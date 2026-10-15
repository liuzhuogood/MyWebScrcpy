#!/usr/bin/env python3
"""Generate and test XPath 1.0 selectors against a local UI dump."""

import argparse
import json
from pathlib import Path
import sys
from xml.etree.ElementTree import ParseError

from lxml import etree

from analyze_dump import analyze


def literal(value: str) -> str:
    if "'" not in value:
        return f"'{value}'"
    if '"' not in value:
        return f'"{value}"'
    return 'concat(' + ', "\'", '.join(f"'{part}'" for part in value.split("'")) + ')'


def predicate(element, fields) -> str:
    parts = []
    for field in fields:
        if field not in element.attrib or not element.attrib[field]:
            continue
        if field == 'text' and element.get('password') == 'true':
            continue
        parts.append(f'@{field}={literal(element.attrib[field])}')
    return ' and '.join(parts)


def features(element):
    for field in ('resource-id', 'text', 'content-desc'):
        value = predicate(element, (field,))
        if value:
            yield value


def evaluate(root, expression: str, node_ids: dict) -> dict:
    try:
        matches = root.xpath(expression, smart_strings=False)
    except etree.XPathError as error:
        raise ValueError(f'XPath 无效：{error}') from None
    if not isinstance(matches, list) or any(
            not isinstance(node, etree._Element) or node.tag != 'node' for node in matches):
        raise ValueError('XPath 必须返回 UI node 元素集合，不能返回属性、字符串或标量')
    ids = [node_ids[node] for node in matches]
    return {'expression': expression, 'match_count': len(ids),
            'unique': len(ids) == 1, 'matched_node_ids': ids}


def candidate_expressions(element):
    # Direct semantic fields first; class/package narrow existing conditions.
    for fields in (('resource-id',), ('text',), ('resource-id', 'text')):
        base = predicate(element, fields)
        if not base:
            continue
        yield f'//node[{base}]', 'id_text', 'semantic'
        for extra in (('package',), ('class',), ('package', 'class')):
            scope = predicate(element, extra)
            if scope:
                yield f'//node[{base} and {scope}]', 'id_text_scoped', 'semantic'

    target = predicate(element, ('resource-id', 'text', 'content-desc', 'class', 'package'))
    target_step = f'//node[{target}]' if target else '//node'
    own_features = set(features(element))
    for axis, sibling in (('preceding-sibling', element.getprevious()),
                          ('following-sibling', element.getnext())):
        if sibling is None:
            continue
        for feature in list(dict.fromkeys(
                feature for node in sibling.iter('node') for feature in features(node)))[:8]:
            if feature in own_features:
                continue
            relation = f'{axis}::node[1][descendant-or-self::node[{feature}]]'
            yield f'{target_step}[{relation}]', 'adjacent_feature', 'relation'

    ancestor = element.getparent()
    for depth in range(1, 4):
        if ancestor is None or ancestor.tag != 'node':
            break
        anchors = list(dict.fromkeys(
            feature for node in ancestor.iter('node') if node is not element
            and element not in node.iterancestors() for feature in features(node)))[:8]
        for feature in features(ancestor):
            if feature in own_features:
                continue
            yield (f'{target_step}[ancestor::node[{depth}][{feature}]]',
                   'ancestor_feature', 'relation')
        for feature in anchors:
            if feature in own_features:
                continue
            yield (f'{target_step}[ancestor::node[{depth}][descendant::node[{feature}]]]',
                   'container_feature', 'relation')
        ancestor = ancestor.getparent()

    description = predicate(element, ('content-desc',))
    if description:
        yield f'//node[{description}]', 'content_desc', 'semantic'
    bounds = predicate(element, ('bounds',))
    if bounds:
        yield f'//node[{bounds}]', 'bounds', 'geometry'
        if target:
            yield f'{target_step}[{bounds}]', 'bounds_scoped', 'geometry'


def generate(root, element, node_ids: dict, geometry_valid: bool) -> dict:
    tested, seen = [], set()
    for expression, strategy, basis in candidate_expressions(element):
        if basis == 'geometry' and not geometry_valid:
            continue
        if expression in seen:
            continue
        seen.add(expression)
        result = evaluate(root, expression, node_ids)
        result.update(strategy=strategy, basis=basis,
                      verified_target=result['matched_node_ids'] == [node_ids[element]])
        tested.append(result)
        if result['verified_target']:
            return {'node_id': node_ids[element], 'selected_xpath': result,
                    'tested_candidates': tested}
    return {'node_id': node_ids[element], 'selected_xpath': None, 'tested_candidates': tested}


def run(response: dict, serial: str, *, expressions=(), target_id=None, page_before=None,
        page_after=None) -> dict:
    analysis = analyze(response, serial)
    parser = etree.XMLParser(resolve_entities=False, no_network=True, load_dtd=False)
    root = etree.fromstring(response['xml'].encode('utf-8'), parser)
    elements = list(root.iter('node'))
    node_ids = {element: index for index, element in enumerate(elements)}
    if target_id is not None and not 0 <= target_id < len(elements):
        raise ValueError('node-id 不在当前 dump 的节点范围内')
    report = {'serial': analysis['serial'], 'captured_at': analysis['captured_at'],
              'summary': analysis['summary'], 'warnings': analysis['warnings']}
    if (page_before is None) != (page_after is None):
        raise ValueError('页面信息前后快照必须一起提供')
    if page_before is not None:
        from page_context import correlate
        report['page_context'] = correlate(analysis, page_before, page_after)
    if expressions:
        tests = [evaluate(root, expression, node_ids) for expression in expressions]
        if target_id is not None:
            for test in tests:
                test['verified_target'] = test['matched_node_ids'] == [target_id]
        report['xpath_tests'] = tests
    else:
        ids = [target_id] if target_id is not None else range(len(elements))
        report['xpath_locators'] = [generate(root, elements[index], node_ids,
            analysis['nodes'][index]['bounds_issue'] is None) for index in ids]
        report['unresolved_nodes'] = [locator['node_id'] for locator in report['xpath_locators']
                                      if locator['selected_xpath'] is None]
    return report


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('response', type=Path, help='当前 /api/ui/xml 原始 JSON 响应')
    parser.add_argument('--expected-serial', required=True)
    parser.add_argument('--node-id', type=int, help='本次分析节点编号；指定时验证目标身份')
    parser.add_argument('--xpath', action='append', default=[], help='测试表达式，可重复传入')
    parser.add_argument('--output', type=Path)
    parser.add_argument('--page-info-before', type=Path)
    parser.add_argument('--page-info-after', type=Path)
    options = parser.parse_args()
    if bool(options.page_info_before) != bool(options.page_info_after):
        parser.error('页面信息前后快照必须一起提供')
    try:
        response = json.loads(options.response.read_text(encoding='utf-8'))
        report = run(response, options.expected_serial, expressions=options.xpath,
                     target_id=options.node_id,
                     page_before=json.loads(options.page_info_before.read_text(encoding='utf-8'))
                     if options.page_info_before else None,
                     page_after=json.loads(options.page_info_after.read_text(encoding='utf-8'))
                     if options.page_info_after else None)
        text = json.dumps(report, ensure_ascii=False, indent=2) + '\n'
        if options.output:
            options.output.write_text(text, encoding='utf-8')
        else:
            sys.stdout.write(text)
        if options.xpath:
            valid = all(test['unique'] and (
                options.node_id is None or test['verified_target']) for test in report['xpath_tests'])
        else:
            valid = bool(report['xpath_locators']) and not report['unresolved_nodes']
        if not valid:
            print('[xpath] 存在零匹配、多匹配、目标不符或无唯一定位结果', file=sys.stderr)
        return 0 if valid else 1
    except (ValueError, OSError, ParseError, etree.LxmlError) as error:
        print(f'[xpath] 失败：{error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
