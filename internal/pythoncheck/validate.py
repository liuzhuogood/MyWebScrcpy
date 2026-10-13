"""Validate SDK generation contracts without executing generated code."""
import ast
import inspect
import json
import sys

IMPORTS = {"argparse", "os", "time", "shlex"}
BUILTINS = {"print", "str", "int", "float", "len", "range", "bool", "abs", "min", "max", "RuntimeError", "SystemExit"}
ENV_KEYS = {"MYWEBSCRCPY_API_URL", "MYWEBSCRCPY_API_TOKEN"}
METHODS = {"adb": "args, timeout=30", "page_info": "timeout=10"}


def validate(source):
    tree = ast.parse(source)
    compile(tree, "generated.py", "exec")
    imports = [n for n in tree.body if isinstance(n, (ast.Import, ast.ImportFrom))]
    sdk = [n for n in imports if isinstance(n, ast.ImportFrom)]
    if len(sdk) != 1 or sdk[0].module != "mywebscrcpy" or sdk[0].level or len(sdk[0].names) != 1 or sdk[0].names[0].name != "Client" or sdk[0].names[0].asname:
        raise ValueError("必须直接导入 from mywebscrcpy import Client")
    for statement in imports:
        if isinstance(statement, ast.Import) and any(n.name not in IMPORTS or n.asname for n in statement.names):
            raise ValueError("仅允许 argparse、os、time、shlex 标准库")
    functions = [n for n in tree.body if isinstance(n, ast.FunctionDef)]
    if len(functions) != 1 or functions[0].name != "main":
        raise ValueError("必须提供唯一 main()")
    main = functions[0]
    if main.decorator_list or ast.dump(main.args) != ast.dump(ast.parse("def main(): pass").body[0].args):
        raise ValueError("入口必须是无装饰器的 main()")
    expected = ast.parse('if __name__ == "__main__":\n    main()').body[0]
    if tree.body != imports + [main, tree.body[-1]] or ast.dump(tree.body[-1]) != ast.dump(expected):
        raise ValueError("顶层仅允许导入、main 与标准启动入口")
    signatures = {}
    for name, params in METHODS.items():
        namespace = {}
        # This trusted signature is a validator constant, never generated code.
        exec(f"def method({params}): pass", namespace)
        signatures[name] = inspect.signature(namespace["method"])
    statements = list(ast.walk(main))
    client_scope = device_scope = False
    for node in statements:
        if isinstance(node, (ast.Import, ast.ImportFrom, ast.ClassDef, ast.AsyncFunctionDef, ast.Lambda,
                             ast.While, ast.AsyncWith, ast.Try, ast.Global, ast.Nonlocal,
                             ast.Delete, ast.Yield, ast.YieldFrom, ast.Await, ast.NamedExpr)):
            raise ValueError(f"不支持的脚本结构：{type(node).__name__}")
        if isinstance(node, ast.FunctionDef) and node is not main:
            raise ValueError("不生成额外封装函数")
        if isinstance(node, ast.With):
            for item in node.items:
                if not isinstance(item.context_expr, ast.Call) or not isinstance(item.optional_vars, ast.Name):
                    raise ValueError("with 仅用于 SDK 资源生命周期")
                call, target = item.context_expr, item.optional_vars.id
                if isinstance(call.func, ast.Name) and call.func.id == "Client" and target == "client":
                    client_scope = True
                elif ast.unparse(call.func) == "client.device" and target == "device":
                    device_scope = True
                else:
                    raise ValueError("with 仅允许 Client/client.device 上下文")
        if isinstance(node, ast.Name):
            if node.id.startswith("_") or isinstance(node.ctx, ast.Store) and node.id in IMPORTS | BUILTINS | {"Client", "main"}:
                raise ValueError("不允许内部名称或重写导入/入口")
        if isinstance(node, ast.Assign) and any(isinstance(t, ast.Name) and t.id in {"client", "device"} for t in node.targets):
            raise ValueError("不能重写已绑定的 client/device")
        if isinstance(node, ast.Attribute):
            if node.attr.startswith("_") or isinstance(node.ctx, ast.Store):
                raise ValueError("不能访问内部或覆盖对象属性")
            if not isinstance(node.value, ast.Name) and ast.unparse(node.value) != "os.environ":
                raise ValueError("不允许嵌套对象属性")
            root = ast.unparse(node.value)
            allowed = {
                "os": {"environ"}, "os.environ": {"get"}, "argparse": {"ArgumentParser"},
                "time": {"sleep"}, "shlex": {"quote"}, "client": {"device"}, "device": set(METHODS),
            }
            local = {"serial", "base_url", "token", "add_argument", "parse_args", "stdout", "stderr", "exit_code", "status", "truncated", "width", "height"}
            if node.attr not in allowed.get(root, local):
                raise ValueError(f"不支持的属性：{root}.{node.attr}")
        if isinstance(node, ast.Call):
            if any(isinstance(a, ast.Starred) for a in node.args) or any(k.arg is None for k in node.keywords):
                raise ValueError("不允许动态展开调用参数")
            if isinstance(node.func, ast.Name):
                if node.func.id not in BUILTINS | {"Client"}:
                    raise ValueError("调用仅允许 SDK 和基础库")
            elif not isinstance(node.func, ast.Attribute):
                raise ValueError("不支持间接调用")
            name = ast.unparse(node.func)
            if name == "Client" and (len(node.args) != 1 or ast.unparse(node.args[0]) != "options.base_url" or any(k.arg not in {"headers", "timeout"} for k in node.keywords)):
                raise ValueError("Client 必须明确地址，不得关闭 TLS 验证")
            if name == "client.device" and (len(node.args) != 1 or ast.unparse(node.args[0]) != "options.serial" or node.keywords):
                raise ValueError("client.device 必须传入单个设备参数")
            if isinstance(node.func, ast.Attribute) and isinstance(node.func.value, ast.Name) and node.func.value.id == "device":
                signatures[node.func.attr].bind(*node.args, **{k.arg: k.value for k in node.keywords})
            if name == "os.environ.get" and (not node.args or not isinstance(node.args[0], ast.Constant) or node.args[0].value not in ENV_KEYS):
                raise ValueError("只能读取服务地址和本次连接令牌")
            if name == "time.sleep" and (len(node.args) != 1 or not isinstance(node.args[0], ast.Constant) or type(node.args[0].value) not in {int, float} or not 0 <= node.args[0].value <= 600):
                raise ValueError("sleep 必须为 0 到 600 秒的明确等待")
        if isinstance(node, ast.Subscript) and ast.unparse(node.value) == "os.environ":
            if not isinstance(node.slice, ast.Constant) or node.slice.value not in ENV_KEYS:
                raise ValueError("只能读取连接环境变量")
        if isinstance(node, (ast.For, ast.comprehension)):
            iterable = node.iter
            if not isinstance(iterable, ast.Call) or ast.unparse(iterable.func) != "range" or iterable.keywords:
                raise ValueError("循环必须采用常量整数 range")
            if not 1 <= len(iterable.args) <= 3 or any(not isinstance(v, ast.Constant) or type(v.value) is not int for v in iterable.args):
                raise ValueError("range 必须采用常量整数")
            if len(range(*(v.value for v in iterable.args))) > 1000:
                raise ValueError("单层循环不能超过 1000 次")
    if not client_scope or not device_scope:
        raise ValueError("必须通过 with Client/client.device 管理资源")


try:
    validate(sys.stdin.read(256 * 1024 + 1))
    print(json.dumps({"ok": True}))
except Exception as error:
    print(json.dumps({"ok": False, "error": str(error)}, ensure_ascii=False))
    sys.exit(1)
