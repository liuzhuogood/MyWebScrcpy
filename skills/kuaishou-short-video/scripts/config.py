#!/usr/bin/env python3
"""快手极速版：配置管理器

读取 page_rules.yaml 中的四区域定义与各页面文字规则：
- 可信度 (confidence: 0.0 ~ 1.0)
- 归属区域 (regions: [1, 2, 3, 4])
- 文字特征 (words: ["文字1", "文字2", ...])
"""

from dataclasses import dataclass, field
from enum import Enum
import os
from pathlib import Path
from typing import Any
import yaml


class PageType(str, Enum):
    """快手极速版页面枚举"""
    VIDEO_FEED = "刷视频页"
    TASK_CENTER = "任务中心"
    AD = "广告页"
    LIVE = "直播页"
    DIALOG = "弹窗页"
    UNKNOWN = "未知页面"


# 屏幕四个区域定义 (Y 轴归一化区间 0.0 ~ 1.0)
DEFAULT_REGION_CONFIG = {
    1: {"name": "顶部区域", "bounds": [0.00, 0.20]},
    2: {"name": "中上区域", "bounds": [0.20, 0.50]},
    3: {"name": "中下区域", "bounds": [0.50, 0.80]},
    4: {"name": "底部区域", "bounds": [0.80, 1.00]},
}


@dataclass
class RuleItem:
    words: list[str] = field(default_factory=list)
    templates: list[str] = field(default_factory=list)
    regions: list[int] = field(default_factory=list)
    confidence: float = 0.5


@dataclass
class PatternItem:
    regex: str = ""
    confidence: float = 0.5


@dataclass
class PageRuleConfig:
    priority: bool = False
    rules: list[RuleItem] = field(default_factory=list)
    patterns: list[PatternItem] = field(default_factory=list)


CONFIG_DIR = Path(__file__).resolve().parent
DEFAULT_YAML_PATH = CONFIG_DIR / "page_rules.yaml"


def load_yaml_config(yaml_path: Path | str | None = None) -> dict[str, Any]:
    """读取并解析 YAML 配置文件，若不存在则返回默认空结构"""
    path = Path(yaml_path) if yaml_path else DEFAULT_YAML_PATH
    if not path.is_file():
        return {}
    try:
        with open(path, "r", encoding="utf-8") as f:
            return yaml.safe_load(f) or {}
    except Exception as e:
        print(f"[警告] 读取规则配置 {path} 失败: {e}，将使用内置默认配置", flush=True)
        return {}


class ConfigManager:
    """页面规则配置管理器"""

    def __init__(self, yaml_path: Path | str | None = None):
        self.yaml_path = Path(yaml_path) if yaml_path else DEFAULT_YAML_PATH
        self.raw_data = load_yaml_config(self.yaml_path)
        self.regions = self._parse_regions()
        self.pages = self._parse_pages()

    def reload(self):
        """重新加载 YAML 配置"""
        self.raw_data = load_yaml_config(self.yaml_path)
        self.regions = self._parse_regions()
        self.pages = self._parse_pages()

    def _parse_regions(self) -> dict[int, dict[str, Any]]:
        reg_data = self.raw_data.get("regions") or DEFAULT_REGION_CONFIG
        parsed = {}
        for k, v in reg_data.items():
            parsed[int(k)] = {
                "name": v.get("name", f"区域{k}"),
                "bounds": [float(v["bounds"][0]), float(v["bounds"][1])],
            }
        return parsed

    def _parse_pages(self) -> dict[str, PageRuleConfig]:
        pages_dict: dict[str, PageRuleConfig] = {}
        raw_pages = self.raw_data.get("pages") or {}

        for page_name, conf in raw_pages.items():
            rule_items = []
            for r in conf.get("rules", []):
                words = r.get("words", [])
                templates = r.get("template") or r.get("templates") or []
                if isinstance(templates, str):
                    templates = [templates]
                rule_items.append(
                    RuleItem(
                        words=words,
                        templates=templates,
                        regions=[int(x) for x in r.get("regions", [])],
                        confidence=float(r.get("confidence", 0.5)),
                    )
                )
            pattern_items = []
            for p in conf.get("patterns", []):
                pattern_items.append(
                    PatternItem(
                        regex=p.get("regex", ""),
                        confidence=float(p.get("confidence", 0.5)),
                    )
                )
            pages_dict[page_name] = PageRuleConfig(
                priority=bool(conf.get("priority", False)),
                rules=rule_items,
                patterns=pattern_items,
            )
        return pages_dict

    def locate_box_region(self, box: dict[str, Any]) -> int:
        """根据 OCR 文本框的 Y 轴中心归一化坐标计算所属区域编号 (1, 2, 3, 4)"""
        y = float(box.get("y", 0.0))
        h = float(box.get("h", 0.0))
        center_y = y + (h / 2.0)

        for reg_id, info in self.regions.items():
            b_min, b_max = info["bounds"]
            # 最后一个区域包含 1.0 上界
            if reg_id == 4:
                if b_min <= center_y <= (b_max + 0.01):
                    return 4
            elif b_min <= center_y < b_max:
                return reg_id
        return 2  # 兜底到中上区

    def get_region_name(self, region_id: int) -> str:
        return self.regions.get(region_id, {}).get("name", f"区域{region_id}")


# 单例配置
config = ConfigManager()
