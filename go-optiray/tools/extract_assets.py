"""Build-time extraction only. The Go executable never runs Python."""
import ast
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / 'releases' / '4.0.2'
TARGET = ROOT / 'go-optiray' / 'assets'
TARGET.mkdir(parents=True, exist_ok=True)
spec = importlib.util.spec_from_file_location('optiray_ui', SOURCE / 'web_ui.py')
ui = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ui)
(TARGET / 'page.html').write_text(ui.HTML_TPL.replace('<style>__CSS__</style>', '<link rel="stylesheet" href="/assets/style.css">').replace('__CSS__', '@import url("/assets/style.css");').replace('__JS__', ''), encoding='utf-8')
page = (TARGET / 'page.html').read_text(encoding='utf-8')
page = page.replace('</body>', '<script src="/assets/app.js"></script></body>')
(TARGET / 'page.html').write_text(page, encoding='utf-8')
(TARGET / 'style.css').write_text(ui.CSS, encoding='utf-8')
(TARGET / 'app.js').write_text(ui.JS.replace('__PALETTE__', json.dumps(ui.DEFAULT_PALETTE)), encoding='utf-8')
tree = ast.parse((SOURCE / 'app.py').read_text(encoding='utf-8-sig'))
default_fn = next(n for n in tree.body if isinstance(n, ast.FunctionDef) and n.name == '_make_default_data')
scope = {'SCHEMA_VERSION': 3, 'DEFAULT_PALETTE': ui.DEFAULT_PALETTE, 'get_persistent_hwid': lambda: ''}
exec(compile(ast.Module(body=[default_fn], type_ignores=[]), '<defaults>', 'exec'), scope)
(TARGET / 'defaults.json').write_text(json.dumps(scope['_make_default_data'](), ensure_ascii=False, indent=2), encoding='utf-8')
routes = []
for node in ast.walk(tree):
    if isinstance(node, ast.Compare):
        for value in node.comparators:
            if isinstance(value, ast.Constant) and isinstance(value.value, str) and value.value.startswith(('/api/', '/manage', '/select')):
                routes.append(value.value)
(TARGET / 'route-contract.json').write_text(json.dumps(sorted(set(routes)), indent=2), encoding='utf-8')
print('Extracted UI, defaults, and route contract from 4.0.2')
