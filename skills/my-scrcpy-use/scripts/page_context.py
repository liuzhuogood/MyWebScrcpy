"""Correlate page-info observations with a UI dump; no device calls."""

from datetime import datetime


def timestamp(value: str) -> datetime:
    try:
        result = datetime.fromisoformat(value.replace('Z', '+00:00'))
    except ValueError:
        raise ValueError('采集时间不是有效 ISO 日期时间') from None
    if result.utcoffset() is None:
        raise ValueError('采集时间必须包含时区')
    return result


def validate_page_info(value: dict) -> None:
    if not isinstance(value, dict):
        raise ValueError('page-info 响应必须是对象')
    for key in ('package_name', 'activity', 'component', 'window_name', 'fetched_at'):
        if key not in value or not isinstance(value[key], str):
            raise ValueError(f'page-info 缺少有效的 {key} 字段；请核对当前接口定义')
    timestamp(value['fetched_at'])
    if 'display' in value:
        display = value['display']
        if not isinstance(display, dict):
            raise ValueError('page-info 的 display 必须是对象')
        for key in ('width', 'height', 'density_dpi'):
            if key in display and (type(display[key]) is not int or display[key] <= 0):
                raise ValueError(f'page-info 的 display.{key} 必须是正整数')
        if 'rotation' in display and (type(display['rotation']) is not int
                                      or display['rotation'] not in (0, 90, 180, 270)):
            raise ValueError('page-info 的 display.rotation 不在已定义的旋转角度中')


def correlate(analysis: dict, before: dict, after: dict) -> dict:
    validate_page_info(before)
    validate_page_info(after)
    started, captured, finished = (timestamp(before['fetched_at']),
                                   timestamp(analysis['captured_at']), timestamp(after['fetched_at']))
    within_window = started <= captured <= finished
    fields = ('package_name', 'activity', 'component', 'window_name')
    changed = [key for key in fields if before[key] != after[key]]
    display_changes = []
    for key in ('width', 'height', 'density_dpi', 'rotation'):
        if ('display' in before and key in before['display']
                and 'display' in after and key in after['display']
                and before['display'][key] != after['display'][key]):
            display_changes.append(key)
    dimensions = None
    if analysis['display_size'] is not None and 'display' in after:
        if all(key in after['display'] for key in ('width', 'height')):
            dimensions = all(analysis['display_size'][key] == after['display'][key]
                             for key in ('width', 'height'))
    packages = analysis['summary']['packages']
    foreground = after['package_name']
    focus_in_dump = foreground in packages if foreground else None
    warnings = ['page-info 不含 serial；设备对应关系依赖采集请求使用同一 serial。',
                '两个接口不是原子快照；前后信息一致也不能排除中间变化。']
    if dimensions is False:
        warnings.append('页面信息尺寸与 XML 坐标基准不一致，坐标操作需要重新验证。')
    if not within_window:
        status = 'capture_order_invalid'
        warnings.append('XML 时间未落在前后 page-info 之间，不能按同一采集批次判断页面。')
    elif changed or display_changes:
        status = 'page_observations_changed'
        warnings.append('采集前后页面或屏幕信息发生变化，需要重新采集稳定页面。')
    elif not foreground:
        status = 'focus_unknown'
        warnings.append('前台包名为空，不能从 Activity 或 component 猜测替代值。')
    elif not analysis['summary']['node_count']:
        status = 'dump_empty'
        warnings.append('页面焦点已读取，但 XML 没有可用于定位的节点。')
    elif not focus_in_dump:
        status = 'focus_absent_from_dump'
        warnings.append('焦点包名未出现在 XML 中，不能把焦点信息当作业务页面已经可操作的证明。')
    elif dimensions is False:
        status = 'display_size_mismatch'
    else:
        status = 'focus_observed_in_dump'
    return {'status': status, 'before': before, 'after': after,
            'capture_within_page_info_window': within_window,
            'observation_window_seconds': (finished - started).total_seconds(),
            'changed_page_fields': changed, 'changed_display_fields': display_changes,
            'focus_package_in_dump': focus_in_dump, 'display_size_matches': dimensions,
            'warnings': warnings}
