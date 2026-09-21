import argparse
import importlib.machinery
import importlib.util
import io
import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


SCRIPT = Path(__file__).parent / "gsso"
LOADER = importlib.machinery.SourceFileLoader("gsso_module", str(SCRIPT))
SPEC = importlib.util.spec_from_loader(LOADER.name, LOADER)
gsso = importlib.util.module_from_spec(SPEC)
LOADER.exec_module(gsso)


class McpConfigTests(unittest.TestCase):
    def test_json_allowlist_preserves_other_settings(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "mcp.json"
            path.write_text(
                json.dumps(
                    {
                        "mcpServers": {
                            "aws-mcp": {
                                "command": "uvx",
                                "args": ["mcp-proxy-for-aws@latest"],
                                "env": {"OTHER": "kept"},
                            },
                            "other": {"command": "other"},
                        }
                    }
                )
            )

            gsso.write_json_mcp_profiles(path, ["readonly-24g", "prod-24g"])

            data = json.loads(path.read_text())
            self.assertEqual(
                data["mcpServers"]["aws-mcp"]["env"],
                {
                    "OTHER": "kept",
                    "AWS_MCP_PROXY_PROFILES": "readonly-24g prod-24g",
                },
            )
            self.assertEqual(data["mcpServers"]["other"], {"command": "other"})

    def test_json_allowlist_tolerates_trailing_commas(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "mcp.json"
            path.write_text(
                "{\n"
                '  "mcpServers": {\n'
                '    "aws-mcp": {\n'
                '      "command": "uvx",\n'
                '      "args": [\n'
                '        "mcp-proxy-for-aws@latest",\n'
                '        "INSTALL_SOURCE=aws-cli,]",\n'
                "      ],\n"
                '      "env": {\n'
                '        "AWS_MCP_PROXY_PROFILES": "readonly-24g"\n'
                "      }\n"
                "    }\n"
                "  }\n"
                "}\n"
            )

            self.assertEqual(gsso.read_json_mcp_profiles(path), ["readonly-24g"])

            gsso.write_json_mcp_profiles(path, ["readonly-24g", "prod-24g"])

            data = json.loads(path.read_text())
            self.assertEqual(
                data["mcpServers"]["aws-mcp"]["args"],
                ["mcp-proxy-for-aws@latest", "INSTALL_SOURCE=aws-cli,]"],
            )
            self.assertEqual(
                gsso.read_json_mcp_profiles(path), ["readonly-24g", "prod-24g"]
            )

    def test_toml_allowlist_adds_environment_table(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.toml"
            path.write_text(
                '[mcp_servers."aws-mcp"]\n'
                'command = "uvx"\n'
                'args = ["mcp-proxy-for-aws@latest"]\n'
                "\n"
                "[unrelated]\n"
                'value = "kept"\n'
            )

            gsso.write_toml_mcp_profiles(path, ["readonly-24g", "prod-24g"])

            self.assertEqual(
                gsso.read_toml_mcp_profiles(path), ["readonly-24g", "prod-24g"]
            )
            self.assertIn('[unrelated]\nvalue = "kept"', path.read_text())

    def test_toml_allowlist_merges_inline_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.toml"
            path.write_text(
                "[mcp_servers.aws-mcp]\n"
                'command = "uvx"\n'
                'env = { OTHER = "kept" }\n'
            )

            gsso.write_toml_mcp_profiles(path, ["readonly-24g"])

            self.assertEqual(gsso.read_toml_mcp_profiles(path), ["readonly-24g"])
            self.assertIn('OTHER = "kept"', path.read_text())

    def test_numbered_multi_select_accepts_ranges(self):
        with patch.object(gsso, "require_tty"), patch.object(
            gsso, "supports_checkbox_menu", return_value=False
        ), patch("builtins.input", return_value="1,3-4"):
            selected, default = gsso.choose(
                ["one", "two", "three", "four"], "Pick", True
            )
        self.assertEqual(selected, ["one", "three", "four"])
        self.assertIsNone(default)

    def test_numbered_multi_select_marks_default(self):
        with patch.object(gsso, "require_tty"), patch.object(
            gsso, "supports_checkbox_menu", return_value=False
        ), patch("builtins.input", return_value="1,d4,3"):
            selected, default = gsso.choose(
                ["one", "two", "three", "four"], "Pick", True, allow_default=True
            )
        self.assertEqual(selected, ["one", "four", "three"])
        self.assertEqual(default, "four")

    def test_multi_select_uses_native_checkbox_menu_when_supported(self):
        with patch.object(gsso, "require_tty"), patch.object(
            gsso, "supports_checkbox_menu", return_value=True
        ), patch.object(
            gsso, "choose_checkboxes", return_value=(["three", "one"], "one")
        ) as menu:
            selected, default = gsso.choose(
                ["one", "two", "three"], "Pick", True, allow_default=True
            )
        self.assertEqual(selected, ["three", "one"])
        self.assertEqual(default, "one")
        menu.assert_called_once_with(
            ["one", "two", "three"],
            "Pick",
            allow_default=True,
            preselected=(),
            default_option=None,
        )

    def test_checkbox_menu_can_search_and_select_a_match(self):
        master, slave = os.openpty()

        class TerminalInput:
            def fileno(self):
                return slave

        try:
            keys = [
                "search",
                "char:v",
                "char:w",
                "char:g",
                "char:o",
                "char:a",
                "enter",
                "space",
                "enter",
            ]
            with patch.object(gsso.sys, "stdin", TerminalInput()), patch.object(
                gsso.sys, "stderr", io.StringIO()
            ), patch.object(gsso, "read_menu_key", side_effect=keys):
                selected, default = gsso.choose_checkboxes(
                    ["vwgoa-24g", "vwgoa-24g-ViewOnlyAccess", "auth-24g"],
                    "Pick",
                    preselected=["auth-24g"],
                )
        finally:
            os.close(master)
            os.close(slave)

        self.assertEqual(selected, ["auth-24g", "vwgoa-24g"])
        self.assertIsNone(default)

    def test_with_default_first_reorders_without_resorting(self):
        self.assertEqual(
            gsso.with_default_first(["alpha-24g", "readonly-24g", "prod-24g"], "prod-24g"),
            ["prod-24g", "alpha-24g", "readonly-24g"],
        )

    def test_current_mcp_profiles_truncates_to_intersection(self):
        configs = [
            gsso.McpConfig(
                "Cursor", Path("/cursor"), "json", ["prod-24g", "alpha-24g", "readonly-24g"]
            ),
            gsso.McpConfig(
                "Codex", Path("/codex"), "toml", ["prod-24g", "alpha-24g"]
            ),
        ]
        with patch.object(gsso, "err"):
            self.assertEqual(
                gsso.current_mcp_profiles(configs),
                ["prod-24g", "alpha-24g"],
            )

    def test_current_mcp_profiles_empty_intersection_is_empty(self):
        configs = [
            gsso.McpConfig("Cursor", Path("/cursor"), "json", ["prod-24g"]),
            gsso.McpConfig("Codex", Path("/codex"), "toml", ["readonly-24g"]),
        ]
        with patch.object(gsso, "err"):
            self.assertEqual(gsso.current_mcp_profiles(configs), [])

    def test_mcp_add_realigns_drifted_agent_configs(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            cursor = root / "cursor.json"
            codex = root / "config.toml"
            cursor.write_text(
                json.dumps(
                    {
                        "mcpServers": {
                            "aws-mcp": {
                                "command": "uvx",
                                "env": {
                                    "AWS_MCP_PROXY_PROFILES": (
                                        "prod-24g readonly-24g extra-24g"
                                    )
                                },
                            }
                        }
                    }
                )
            )
            codex.write_text(
                "[mcp_servers.aws-mcp]\n"
                'command = "uvx"\n'
                "[mcp_servers.aws-mcp.env]\n"
                'AWS_MCP_PROXY_PROFILES = "prod-24g readonly-24g"\n'
            )
            configs = [
                gsso.McpConfig(
                    "Cursor",
                    cursor,
                    "json",
                    ["prod-24g", "readonly-24g", "extra-24g"],
                ),
                gsso.McpConfig(
                    "Codex",
                    codex,
                    "toml",
                    ["prod-24g", "readonly-24g"],
                ),
            ]
            args = argparse.Namespace(mcp_action="add", profiles=["alpha-24g"])

            with patch.object(gsso, "discover_mcp_configs", return_value=configs), patch.object(
                gsso,
                "completion_switch_targets",
                return_value=["prod-24g", "readonly-24g", "extra-24g", "alpha-24g"],
            ), patch.object(gsso, "MCP_BACKUP_ROOT", root / "backups"):
                result = gsso.cmd_mcp_change(args)

            self.assertEqual(result, gsso.EX_OK)
            self.assertEqual(
                gsso.read_json_mcp_profiles(cursor),
                ["prod-24g", "readonly-24g", "alpha-24g"],
            )
            self.assertEqual(
                gsso.read_toml_mcp_profiles(codex),
                ["prod-24g", "readonly-24g", "alpha-24g"],
            )

    def test_mcp_default_moves_profile_to_front(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            cursor = root / "cursor.json"
            cursor.write_text(
                json.dumps(
                    {
                        "mcpServers": {
                            "aws-mcp": {
                                "command": "uvx",
                                "env": {
                                    "AWS_MCP_PROXY_PROFILES": (
                                        "alpha-24g readonly-24g prod-24g"
                                    )
                                },
                            }
                        }
                    }
                )
            )
            configs = [
                gsso.McpConfig(
                    "Cursor",
                    cursor,
                    "json",
                    ["alpha-24g", "readonly-24g", "prod-24g"],
                )
            ]
            args = argparse.Namespace(profile="readonly-24g")
            with patch.object(gsso, "discover_mcp_configs", return_value=configs), patch.object(
                gsso, "MCP_BACKUP_ROOT", root / "backups"
            ):
                result = gsso.cmd_mcp_default(args)
            self.assertEqual(result, gsso.EX_OK)
            self.assertEqual(
                gsso.read_json_mcp_profiles(cursor),
                ["readonly-24g", "alpha-24g", "prod-24g"],
            )

    def test_mcp_add_updates_all_detected_configs(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            cursor = root / "cursor.json"
            codex = root / "config.toml"
            cursor.write_text(
                json.dumps(
                    {
                        "mcpServers": {
                            "aws-mcp": {"command": "uvx", "args": ["proxy"]}
                        }
                    }
                )
            )
            codex.write_text(
                "[mcp_servers.aws-mcp]\ncommand = \"uvx\"\nargs = [\"proxy\"]\n"
            )
            configs = [
                gsso.McpConfig("Cursor", cursor, "json", []),
                gsso.McpConfig("Codex", codex, "toml", []),
            ]
            args = argparse.Namespace(
                mcp_action="add", profiles=["readonly-24g", "prod-24g"]
            )

            with patch.object(gsso, "discover_mcp_configs", return_value=configs), patch.object(
                gsso,
                "completion_switch_targets",
                return_value=["readonly-24g", "prod-24g"],
            ), patch.object(gsso, "MCP_BACKUP_ROOT", root / "backups"):
                result = gsso.cmd_mcp_change(args)

            self.assertEqual(result, gsso.EX_OK)
            self.assertEqual(
                gsso.read_json_mcp_profiles(cursor), ["readonly-24g", "prod-24g"]
            )
            self.assertEqual(
                gsso.read_toml_mcp_profiles(codex), ["readonly-24g", "prod-24g"]
            )
            self.assertEqual(len(list((root / "backups").iterdir())), 1)

    def test_mcp_remove_updates_default_and_keeps_one_profile(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            cursor = root / "cursor.json"
            cursor.write_text(
                json.dumps(
                    {
                        "mcpServers": {
                            "aws-mcp": {
                                "command": "uvx",
                                "env": {
                                    "AWS_MCP_PROXY_PROFILES": (
                                        "readonly-24g prod-24g"
                                    )
                                },
                            }
                        }
                    }
                )
            )
            configs = [
                gsso.McpConfig(
                    "Cursor", cursor, "json", ["readonly-24g", "prod-24g"]
                )
            ]
            args = argparse.Namespace(
                mcp_action="remove", profiles=["readonly-24g"]
            )

            with patch.object(gsso, "discover_mcp_configs", return_value=configs), patch.object(
                gsso,
                "completion_switch_targets",
                return_value=["readonly-24g", "prod-24g"],
            ), patch.object(gsso, "MCP_BACKUP_ROOT", root / "backups"):
                result = gsso.cmd_mcp_change(args)

            self.assertEqual(result, gsso.EX_OK)
            self.assertEqual(gsso.read_json_mcp_profiles(cursor), ["prod-24g"])

    def test_mcp_add_menu_lists_every_profile_with_allowed_ones_checked(self):
        config = gsso.McpConfig(
            "Cursor", Path("/unused"), "json", ["prod-24g", "readonly-24g"]
        )
        args = argparse.Namespace(mcp_action="add", profiles=[])
        with patch.object(gsso, "discover_mcp_configs", return_value=[config]), patch.object(
            gsso,
            "completion_switch_targets",
            return_value=["alpha-24g", "prod-24g", "readonly-24g"],
        ), patch.object(
            gsso, "choose", return_value=(["prod-24g", "readonly-24g"], None)
        ) as menu, patch.object(gsso, "write_mcp_profiles") as write:
            result = gsso.cmd_mcp_change(args)

        self.assertEqual(result, gsso.EX_OK)
        self.assertEqual(
            menu.call_args.args[0], ["alpha-24g", "prod-24g", "readonly-24g"]
        )
        self.assertEqual(
            menu.call_args.kwargs["preselected"], ["prod-24g", "readonly-24g"]
        )
        self.assertEqual(menu.call_args.kwargs["default_option"], "prod-24g")
        write.assert_not_called()

    def test_mcp_add_menu_unchecking_removes_and_keeps_default(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            cursor = root / "cursor.json"
            cursor.write_text(
                json.dumps(
                    {
                        "mcpServers": {
                            "aws-mcp": {
                                "command": "uvx",
                                "env": {
                                    "AWS_MCP_PROXY_PROFILES": (
                                        "prod-24g readonly-24g"
                                    )
                                },
                            }
                        }
                    }
                )
            )
            configs = [
                gsso.McpConfig(
                    "Cursor", cursor, "json", ["prod-24g", "readonly-24g"]
                )
            ]
            args = argparse.Namespace(mcp_action="add", profiles=[])

            with patch.object(gsso, "discover_mcp_configs", return_value=configs), patch.object(
                gsso,
                "completion_switch_targets",
                return_value=["alpha-24g", "prod-24g", "readonly-24g"],
            ), patch.object(
                gsso, "choose", return_value=(["prod-24g", "alpha-24g"], None)
            ), patch.object(gsso, "MCP_BACKUP_ROOT", root / "backups"):
                result = gsso.cmd_mcp_change(args)

            self.assertEqual(result, gsso.EX_OK)
            self.assertEqual(
                gsso.read_json_mcp_profiles(cursor), ["prod-24g", "alpha-24g"]
            )

    def test_mcp_remove_refuses_to_empty_allowlist(self):
        config = gsso.McpConfig(
            "Cursor", Path("/unused"), "json", ["readonly-24g"]
        )
        args = argparse.Namespace(
            mcp_action="remove", profiles=["readonly-24g"]
        )
        with patch.object(gsso, "discover_mcp_configs", return_value=[config]), patch.object(
            gsso, "completion_switch_targets", return_value=["readonly-24g"]
        ):
            with self.assertRaises(gsso.GssoError):
                gsso.cmd_mcp_change(args)

    def test_configure_preselects_managed_roles_and_keeps_stale_roles_visible(self):
        profiles = [
            gsso.Profile(
                "acme-read",
                "111122223333",
                "Acme",
                "ReadOnly",
                managed=True,
            ),
            gsso.Profile(
                "legacy-admin",
                "444455556666",
                "Legacy",
                "Admin",
                managed=True,
            ),
        ]
        remote = [
            ("111122223333", "Acme", "ReadOnly"),
            ("111122223333", "Acme", "Developer"),
        ]
        args = argparse.Namespace(hard_refresh=False)

        def keep_current(options, _prompt, multiple, preselected):
            self.assertTrue(multiple)
            self.assertIn("Acme (111122223333)/ReadOnly", options)
            self.assertIn("Acme (111122223333)/Developer", options)
            self.assertIn(
                "Legacy (444455556666)/Admin (not currently available)", options
            )
            self.assertEqual(
                set(preselected),
                {
                    "Acme (111122223333)/ReadOnly",
                    "Legacy (444455556666)/Admin (not currently available)",
                },
            )
            return list(preselected), None

        with patch.object(gsso, "require_aws"), patch.object(
            gsso, "load_remote_pairs", return_value=remote
        ), patch.object(
            gsso, "read_config_profiles", return_value=profiles
        ), patch.object(
            gsso, "choose", side_effect=keep_current
        ), patch.object(
            gsso, "create_rollback_point"
        ) as backup:
            result = gsso.cmd_configure(args)

        self.assertEqual(result, gsso.EX_OK)
        backup.assert_not_called()

    def test_configure_adds_and_removes_roles_with_one_rollback(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config_path = root / "config"
            config_path.write_text(
                "[profile acme-read]\n"
                "# managed-by: 24g-sso-setup\n"
                "# account-name: Acme\n"
                "sso_session = 24g\n"
                "sso_account_id = 111122223333\n"
                "sso_role_name = ReadOnly\n"
                "region = us-east-1\n"
                "output = json\n"
            )
            remote = [
                ("111122223333", "Acme", "ReadOnly"),
                ("111122223333", "Acme", "Developer"),
            ]
            args = argparse.Namespace(hard_refresh=False)

            with patch.object(gsso, "CONFIG_PATH", config_path), patch.object(
                gsso, "require_aws"
            ), patch.object(
                gsso, "load_remote_pairs", return_value=remote
            ), patch.object(
                gsso,
                "choose",
                return_value=(["Acme (111122223333)/Developer"], None),
            ), patch.object(
                gsso, "create_rollback_point", return_value=root / "backup"
            ) as backup, patch.object(
                gsso, "prompt_profile_name", return_value="custom-dev"
            ) as profile_name, patch.object(
                gsso, "write_profiles"
            ) as write:
                result = gsso.cmd_configure(args)

            self.assertEqual(result, gsso.EX_OK)
            backup.assert_called_once_with()
            self.assertNotIn("[profile acme-read]", config_path.read_text())
            profile_name.assert_called_once_with(
                "Acme/Developer",
                "Acme-Developer",
                set(),
            )
            write.assert_called_once_with(
                [("custom-dev", "111122223333", "Acme", "Developer")],
                "us-east-1",
            )

    def test_add_alias_names_a_single_profile(self):
        args = argparse.Namespace(
            hard_refresh=False,
            targets=["Acme/deploy"],
            alias="acme-production",
        )
        remote = [("111122223333", "Acme", "deploy")]

        with patch.object(gsso, "require_aws"), patch.object(
            gsso, "load_remote_pairs", return_value=remote
        ), patch.object(
            gsso, "read_config_profiles", return_value=[]
        ), patch.object(
            gsso, "create_rollback_point", return_value=Path("/backup")
        ), patch.object(
            gsso, "write_profiles"
        ) as write:
            result = gsso.cmd_add(args)

        self.assertEqual(result, gsso.EX_OK)
        write.assert_called_once_with(
            [("acme-production", "111122223333", "Acme", "deploy")],
            "us-east-1",
        )

    def test_write_profiles_appends_every_section_in_one_save(self):
        with tempfile.TemporaryDirectory() as directory:
            config_path = Path(directory) / "config"
            config_path.write_text(
                "[profile hand-written]\nregion = us-west-2\n"
            )
            entries = [
                ("acme-admin", "111122223333", "Acme", "AdministratorAccess"),
                ("beta-admin", "444455556666", "Beta", "AdministratorAccess"),
            ]

            with patch.object(gsso, "CONFIG_PATH", config_path), patch.object(
                gsso, "aws_configure_set"
            ) as configure_set:
                gsso.write_profiles(entries, "us-east-1")
                names = {p.name for p in gsso.read_config_profiles() if p.managed}

            configure_set.assert_not_called()
            self.assertEqual(names, {"acme-admin", "beta-admin"})
            self.assertEqual(
                config_path.read_text(),
                "[profile hand-written]\n"
                "region = us-west-2\n"
                "\n"
                "[profile acme-admin]\n"
                "# managed-by: 24g-sso-setup\n"
                "# account-name: Acme\n"
                "sso_session = 24g\n"
                "sso_account_id = 111122223333\n"
                "sso_role_name = AdministratorAccess\n"
                "region = us-east-1\n"
                "output = json\n"
                "\n"
                "[profile beta-admin]\n"
                "# managed-by: 24g-sso-setup\n"
                "# account-name: Beta\n"
                "sso_session = 24g\n"
                "sso_account_id = 444455556666\n"
                "sso_role_name = AdministratorAccess\n"
                "region = us-east-1\n"
                "output = json\n",
            )

    def test_write_profiles_refuses_to_shadow_an_existing_profile(self):
        with tempfile.TemporaryDirectory() as directory:
            config_path = Path(directory) / "config"
            config_path.write_text("[profile acme-admin]\nregion = us-west-2\n")

            with patch.object(gsso, "CONFIG_PATH", config_path):
                with self.assertRaises(gsso.GssoError):
                    gsso.write_profiles(
                        [("acme-admin", "111122223333", "Acme", "Admin")],
                        "us-east-1",
                    )

            self.assertEqual(
                config_path.read_text(),
                "[profile acme-admin]\nregion = us-west-2\n",
            )

    def test_add_alias_requires_exactly_one_target(self):
        args = argparse.Namespace(
            hard_refresh=False,
            targets=["Acme/deploy", "Acme/read"],
            alias="acme",
        )
        with patch.object(gsso, "require_aws"), self.assertRaises(gsso.GssoError):
            gsso.cmd_add(args)


if __name__ == "__main__":
    unittest.main()
