#!/usr/bin/env python3
"""Generate Bellum changes and complete upstream notes for changed runtime pins."""
import argparse
import json
from pathlib import Path
import re
import subprocess
from urllib.parse import quote


def git(*args):
    return subprocess.check_output(['git', *args], text=True).strip()


def api(path):
    pages = json.loads(subprocess.check_output(
        ['gh', 'api', '--paginate', '--slurp', path], text=True))
    return [release for page in pages for release in page]


def version(tag):
    match = re.fullmatch(r'v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-rc\.[0-9]+)?', tag)
    if not match:
        raise ValueError('invalid release tag: ' + tag)
    return tuple(map(int, match.groups()))


def baseline(tag, releases):
    candidates = [r['tag_name'] for r in releases
                  if not r.get('draft') and not r.get('prerelease')
                  and re.fullmatch(r'v\d+\.\d+\.\d+', r['tag_name'])
                  and version(r['tag_name']) < version(tag)]
    return max(candidates, key=version) if candidates else None


def pins(source):
    fields = dict(re.findall(r'^\s*(\w+):\s*"([^"\n]*)"', source, re.M))
    allowlist = re.search(r'LauncherSHA256Allowlist:\s*\[\]string\{([^}]+)\}', source)
    fields['launcher'] = re.findall(r'"([a-fA-F0-9]{64})"', allowlist[1]) if allowlist else []
    for required in ['ProtonVer', 'ProtonSHA256', 'UMUVersion', 'UMUZipappSHA256', 'WinetricksVer']:
        if not fields.get(required):
            raise ValueError('missing runtime pin: ' + required)
    return fields


def upstream_notes(repo, old, new, releases, eligible):
    # Match by immutable tags, not "latest", so reruns document the shipped pin.
    selected = [r for r in releases if not r.get('draft') and not r.get('prerelease')
                and eligible(r['tag_name'])]
    tags = [r['tag_name'] for r in selected]
    if new not in tags or old not in tags:
        raise ValueError(f'{repo}: cannot establish full release range {old} -> {new}')
    end, start = tags.index(new), tags.index(old)
    if end >= start:
        raise ValueError(f'{repo}: non-forward runtime update {old} -> {new}; review notes manually')
    notes = []
    for release in reversed(selected[end:start]):
        tag = release['tag_name']
        url = f'https://github.com/{repo}/releases/tag/{quote(tag, safe="")}'
        # Keep upstream bodies intact; their embedded links and component updates matter.
        body = release.get('body') or 'Upstream supplied no release notes.'
        notes.append(f'### [{tag}]({url})\n\n{body.strip()}')
    return '\n\n'.join(notes)


def render(current, previous, changes, load=api):
    out = ['## Bellum changes', changes or 'No installer code changes.', '## Runtime pins']
    for label, key, checksum, repo, convert, eligible in [
        ('Proton-CachyOS', 'ProtonVer', 'ProtonSHA256', 'CachyOS/proton-cachyos',
         lambda v: v.removeprefix('proton-').removesuffix('-x86_64'),
         lambda tag: bool(re.fullmatch(r'cachyos-[0-9.]+-[0-9]{8}-slr', tag))),
        ('umu-launcher', 'UMUVersion', 'UMUZipappSHA256', 'Open-Wine-Components/umu-launcher',
         lambda v: v, lambda tag: True),
    ]:
        new, old = current[key], previous.get(key)
        tag = convert(new)
        url = f'https://github.com/{repo}/releases/tag/{quote(tag, safe="")}'
        state = f'`{old}` → [{new}]({url})' if old != new else f'[{new}]({url}) (unchanged)'
        out.append(f'### {label}\n\n{state}\n\nSHA-256: `{current[checksum]}`')
        if old and old != new:
            out.append(upstream_notes(repo, convert(old), tag,
                                      load(f'repos/{repo}/releases?per_page=100'), eligible))
        elif not old:
            releases = load(f'repos/{repo}/releases?per_page=100')
            release = next((r for r in releases if r['tag_name'] == tag), None)
            if release is None:
                raise ValueError(f'{repo}: missing release {tag}')
            out.append(release.get('body') or 'Upstream supplied no release notes.')
        elif previous.get(checksum) != current[checksum]:
            out.append('The content checksum changed at the same version; review the runtime pin change.')
    old = previous.get('WinetricksVer')
    out.append('### Bundled components\n\nDXVK, vkd3d-proton and dxvk-nvapi come from the pinned Proton archive. '
               'Their upstream updates are included in the Proton release notes above when that pin changes.\n\n'
               f'Winetricks: `{current["WinetricksVer"]}`' +
               (f' (previously `{old}`).' if old and old != current['WinetricksVer'] else '.'))
    added = set(current['launcher']) - set(previous.get('launcher', []))
    out.append('### Astarte Launcher\n\n' +
               ('New accepted build SHA-256 values:\n\n' + '\n'.join(f'- `{v}`' for v in sorted(added))
                if added else 'No changes to known launcher build hashes.') +
               '\n\nLauncher downloads remain subject to Authenticode signer verification.')
    return '\n\n'.join(out) + '\n'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--tag', required=True)
    parser.add_argument('--repo', required=True)
    parser.add_argument('--previous', help='Explicit published baseline for local validation')
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    version(args.tag)
    previous = args.previous or baseline(args.tag, api(f'repos/{args.repo}/releases?per_page=100'))
    current = pins(git('show', 'HEAD:pkg/config/versions.go'))
    old = pins(git('show', f'{previous}:pkg/config/versions.go')) if previous else {}
    commits = git('log', '--no-merges', '--format=%H%x09%s', f'{previous}..HEAD' if previous else 'HEAD')
    changes = '\n'.join(f'- {subject} ([{sha[:7]}](https://github.com/{args.repo}/commit/{sha}))'
                        for sha, subject in (line.split('\t', 1) for line in commits.splitlines()))
    text = render(current, old, changes)
    text += f'\nCompare: https://github.com/{args.repo}/compare/{previous}...{args.tag}\n' if previous else ''
    Path(args.output).write_text(text)


if __name__ == '__main__':
    main()
