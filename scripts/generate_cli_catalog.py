"""Snapshot the reference CLI's public command schema; no Python needed at runtime.

Usage: python scripts/generate_cli_catalog.py path/to/notebooklm-mcp-cli
"""
import ast
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1]) / 'src/notebooklm_tools/cli'
modules = {'main': ast.parse((root / 'main.py').read_text(encoding='utf-8'))}
modules.update({p.stem: ast.parse(p.read_text(encoding='utf-8')) for p in (root / 'commands').glob('*.py')})
imports = {}
for module, tree in modules.items():
    for n in tree.body:
        if isinstance(n, ast.ImportFrom) and n.module and '.cli.commands.' in n.module:
            for a in n.names:
                imports[module, a.asname or a.name] = (n.module.rsplit('.', 1)[1], a.name)

def literal(n, fallback=None):
    try:
        v = ast.literal_eval(n)
        return fallback if v is Ellipsis else v
    except (ValueError, TypeError):
        return fallback

def schema(module, n, path):
    args, flags = [], []
    defaults = [None] * (len(n.args.args) - len(n.args.defaults)) + n.args.defaults
    for a, d in zip(n.args.args, defaults):
        if not isinstance(d, ast.Call) or not isinstance(d.func, ast.Attribute) or d.func.attr not in ('Option', 'Argument'):
            continue
        ann = ast.unparse(a.annotation) if a.annotation else 'str'
        kw = {k.arg: k.value for k in d.keywords}
        required = bool(d.args and isinstance(d.args[0], ast.Constant) and d.args[0].value is Ellipsis)
        default = literal(d.args[0]) if d.args else None
        item = {'name': a.arg, 'help': literal(kw.get('help'), ''), 'required': required}
        if d.func.attr == 'Argument':
            item['many'] = 'list[' in ann
            args.append(item)
        else:
            names = [literal(x) for x in d.args[1:] if isinstance(literal(x), str) and literal(x).startswith('-')]
            long = next((x[2:] for x in names if x.startswith('--')), a.arg.replace('_', '-'))
            item.update(name=long, aliases=[x[2:] for x in names if x.startswith('--') and x[2:] != long], short=next((x[1:] for x in names if x.startswith('-') and not x.startswith('--')), ''), type='bool' if 'bool' in ann else 'strings' if 'list[' in ann else 'int' if 'int' in ann else 'float' if 'float' in ann else 'string', default=default)
            flags.append(item)
    return {'path': path, 'canonical': path, 'help': (ast.get_docstring(n) or path).strip().split('\n')[0], 'args': args, 'flags': flags}

commands = []
seen = set()
def walk(module, app, prefix):
    module, app = imports.get((module, app), (module, app))
    if (module, app, prefix) in seen:
        return
    seen.add((module, app, prefix))
    tree = modules[module]
    for n in tree.body:
        if isinstance(n, ast.Expr) and isinstance(n.value, ast.Call):
            c = n.value
            if isinstance(c.func, ast.Attribute) and isinstance(c.func.value, ast.Name) and c.func.value.id == app and c.func.attr == 'add_typer':
                name = next(literal(k.value) for k in c.keywords if k.arg == 'name')
                walk(module, c.args[0].id, (prefix + ' ' + name).strip())
        if not isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef)):
            continue
        for d in n.decorator_list:
            if not (isinstance(d, ast.Call) and isinstance(d.func, ast.Attribute) and isinstance(d.func.value, ast.Name) and d.func.value.id == app):
                continue
            if d.func.attr == 'command':
                path = (prefix + ' ' + (literal(d.args[0]) if d.args else n.name.replace('_', '-'))).strip()
            elif d.func.attr == 'callback' and prefix in ('login', 'doctor', 'usage'):
                path = prefix
            else:
                continue
            item = schema(module, n, path)
            if module == 'verbs':
                for call in ast.walk(n):
                    if isinstance(call, ast.Call) and isinstance(call.func, ast.Name) and (module, call.func.id) in imports:
                        target_module, target_name = imports[module, call.func.id]
                        for fn in modules[target_module].body:
                            if isinstance(fn, ast.FunctionDef) and fn.name == target_name:
                                for dec in fn.decorator_list:
                                    if isinstance(dec, ast.Call) and isinstance(dec.func, ast.Attribute) and dec.func.attr == 'command':
                                        group = target_module
                                        if target_module == 'studio' and dec.func.value.id != 'app':
                                            group = dec.func.value.id.removesuffix('_app').replace('_', '-')
                                        item['canonical'] = group + ' ' + literal(dec.args[0])
            commands.append(item)

walk('main', 'app', '')
commands = [c for c in commands if not c['canonical'].startswith(('setup ', 'skill '))]
for c in commands:
    c['help'] = c['help'].replace('Gemini Notebook MCP installation', 'NotebookLM CLI installation')
    c['flags'] = [f for f in c['flags'] if f['name'] != 'mcp-compatible']
    for f in c['flags']:
        if '/--' in f['name']:
            f['name'], negative = f['name'].split('/--', 1)
            f['negative'] = negative
out = pathlib.Path('internal/cli/command_catalog.json')
out.write_text(json.dumps(sorted(commands, key=lambda c: c['path']), ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
print(f'{len(commands)} command routes written to {out}')
