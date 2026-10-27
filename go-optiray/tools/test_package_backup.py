"""Isolated package-script regression tests; never touches a router."""
import importlib.util
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('feed', Path(__file__).with_name('build-feed.py'))
feed = importlib.util.module_from_spec(spec)
spec.loader.exec_module(feed)


class PackageBackupTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        git = Path(shutil.which('git'))
        self.shell = shutil.which('sh') or str(git.parent.parent / 'usr/bin/sh.exe')
        self.unix = subprocess.check_output([self.shell, '-c', 'pwd'], cwd=self.base, text=True).strip()
        self.root = self.base / 'opt/etc/optiray-go'
        self.root.mkdir(parents=True)
        (self.base / 'tmp').mkdir()
        self.put('.optiray-go-install', 'owned')
        self.put('subs.json', '{"subscriptions":[]}')
        self.put('routing/optirout.json', '{"groups":[]}')
        self.put('routing/backups/keep.json', 'keep')
        self.put('log/engine.log', 'x' * 100000)
        self.put('routing/engine.log.1', 'x' * 100000)
        self.put('xray_releases_cache.json', '{}')
        self.put('run/manager.pid', '123')
        self.binary('1.0.11')
        self.service = self.base / 'opt/etc/init.d/S99optiray-go'
        self.service.parent.mkdir(parents=True)
        self.service.write_text('#!/bin/sh\n# OptiRay Go owned service\nexit 0\n', newline='\n')
        self.service.chmod(0o755)

    def put(self, name, data):
        p = self.root / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(data, newline='\n')

    def binary(self, version, fail=False):
        self.put('VERSION', version)
        self.put('bin/optiray', '#!/bin/sh\ncase "$*" in *--version*) echo ' + version +
                 ';; *--router-address*) echo 192.0.2.1;; *--check-service*) exit ' + str(int(fail)) + ';; esac\n')
        (self.root / 'bin/optiray').chmod(0o755)

    def run_script(self, text, custom=None, free=1000000):
        text = text.replace('/opt/', self.unix + '/opt/').replace(' /tmp/optiray-opkg-state', ' ' + self.unix + '/tmp/optiray-opkg-state')
        text = text.replace('@VERSION@', '1.0.12')
        # Git's minimal Windows shell has no du/df; emulate ample disk space.
        text = text.replace('set -eu', 'set -eu\ntimeout() { shift; "$@"; }\nsync() { :; }\ncurl() { return 7; }\ndu() { echo "4 fixture"; }\ndf() { printf "Filesystem blocks used available capacity mount\\nfixture 1000000 0 ' + str(free) + ' 0 fixture\\n"; }', 1)
        path = self.base / 'script.sh'
        path.write_text(text, newline='\n')
        env = dict(os.environ)
        env['PATH'] = str(Path(self.shell).parent) + os.pathsep + env.get('PATH', '')
        if custom:
            env['OPTIRAY_BACKUP_DIR'] = self.unix + custom
        else:
            env.pop('OPTIRAY_BACKUP_DIR', None)
        return subprocess.run([self.shell, str(path)], cwd=self.base, env=env, capture_output=True, text=True)

    def test_selective_copy_and_success_cleanup(self):
        result = self.run_script(feed.PREINST, '/usb/custom-backup')
        self.assertEqual(result.returncode, 0, result.stderr)
        state = self.base / 'usb/custom-backup'
        self.assertTrue((state / 'root/subs.json').exists())
        self.assertTrue((state / 'root/routing/backups/keep.json').exists())
        for name in ['log/engine.log', 'routing/engine.log.1', 'xray_releases_cache.json', 'run/manager.pid']:
            self.assertFalse((state / 'root' / name).exists(), name)
        self.binary('1.0.12')
        result = self.run_script(feed.POSTINST)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(state.exists())
        self.assertFalse((self.base / 'opt/var/lib/optiray/opkg-backup-path').exists())

    def test_failure_and_reconfigure_preserve_backup(self):
        result = self.run_script(feed.PREINST)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.binary('1.0.12', fail=True)
        result = self.run_script(feed.POSTINST)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.root / 'VERSION').read_text(), '1.0.11')
        state = self.base / 'opt/tmp/optiray-opkg-state'
        self.assertTrue((state / 'complete').exists())
        result = self.run_script(feed.POSTINST)
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue(state.exists())
        self.assertNotEqual(self.run_script(feed.PREINST).returncode, 0)

    def test_space_failure_preserves_application(self):
        result = self.run_script(feed.PREINST, free=1)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Not enough space', result.stderr)
        self.assertTrue((self.root / 'subs.json').exists())
        self.assertFalse((self.base / 'opt/tmp/optiray-opkg-state').exists())

    def test_backup_inside_application_is_refused(self):
        result = self.run_script(feed.PREINST, '/opt/etc/optiray-go/backup')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Unsafe backup path', result.stderr)

    def test_preflight_failure_does_not_stop_service_or_create_backup(self):
        self.put('bin/optiray', '#!/bin/sh\ncase "$*" in *--help*) echo --check-installation;; *--check-installation*) echo "corrupt routing/optirout.json" >&2; exit 1;; esac\n')
        self.service.write_text('#!/bin/sh\n# OptiRay Go owned service\ntouch "' + self.unix + '/stopped"\n', newline='\n')
        result = self.run_script(feed.PREINST)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('routing/optirout.json', result.stderr)
        self.assertFalse((self.base / 'stopped').exists())
        self.assertFalse((self.base / 'opt/tmp/optiray-opkg-state').exists())


if __name__ == '__main__':
    unittest.main()
