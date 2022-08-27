// Copyright 2022 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package android

import (
	"fmt"
	"strings"

	"github.com/google/blueprint/proptools"
)

func init() {
	ctx := InitRegistrationContext
	ctx.RegisterSingletonModuleType("buildinfo_prop", buildinfoPropFactory)
}

type buildinfoPropProperties struct {
	// Whether this module is directly installable to one of the partitions. Default: true.
	Installable *bool
}

type buildinfoPropModule struct {
	SingletonModuleBase

	properties buildinfoPropProperties

	outputFilePath OutputPath
	installPath    InstallPath
}

var _ OutputFileProducer = (*buildinfoPropModule)(nil)

func (p *buildinfoPropModule) installable() bool {
	return proptools.BoolDefault(p.properties.Installable, true)
}

// OutputFileProducer
func (p *buildinfoPropModule) OutputFiles(tag string) (Paths, error) {
	if tag != "" {
		return nil, fmt.Errorf("unsupported tag %q", tag)
	}
	return Paths{p.outputFilePath}, nil
}

func (p *buildinfoPropModule) GenerateAndroidBuildActions(ctx ModuleContext) {
	p.outputFilePath = PathForModuleOut(ctx, p.Name()).OutputPath
	if !ctx.Config().KatiEnabled() {
		WriteFileRule(ctx, p.outputFilePath, "# no buildinfo.prop if kati is disabled")
		return
	}

	rule := NewRuleBuilder(pctx, ctx)
	cmd := rule.Command().Text("(")

	writeString := func(str string) {
		cmd.Text(`echo "` + str + `" && `)
	}

	writeString("# begin build properties")
	writeString("# autogenerated by build/soong/android/buildinfo_prop.go")

	writeProp := func(key, value string) {
		if strings.Contains(key, "=") {
			panic(fmt.Errorf("wrong property key %q: key must not contain '='", key))
		}
		writeString(key + "=" + value)
	}

	config := ctx.Config()

	writeProp("ro.build.version.sdk", config.PlatformSdkVersion().String())
	writeProp("ro.build.version.preview_sdk", config.PlatformPreviewSdkVersion())
	writeProp("ro.build.version.codename", config.PlatformSdkCodename())
	writeProp("ro.build.version.all_codenames", strings.Join(config.PlatformVersionActiveCodenames(), ","))
	writeProp("ro.build.version.release", config.PlatformVersionLastStable())
	writeProp("ro.build.version.release_or_codename", config.PlatformVersionName())
	writeProp("ro.build.version.security_patch", config.PlatformSecurityPatch())
	writeProp("ro.build.version.base_os", config.PlatformBaseOS())
	writeProp("ro.build.version.min_supported_target_sdk", config.PlatformMinSupportedTargetSdkVersion())

	if config.Eng() {
		writeProp("ro.build.type", "eng")
	} else {
		writeProp("ro.build.type", "user")
	}

	// Currently, only a few properties are implemented to unblock microdroid use case.
	// TODO(b/189164487): support below properties as well and replace build/make/tools/buildinfo.sh
	/*
		if $BOARD_USE_VBMETA_DIGTEST_IN_FINGERPRINT {
			writeProp("ro.build.legacy.id", config.BuildID())
		} else {
			writeProp("ro.build.id", config.BuildId())
		}
		writeProp("ro.build.display.id", $BUILD_DISPLAY_ID)
		writeProp("ro.build.version.incremental", $BUILD_NUMBER)
		writeProp("ro.build.version.preview_sdk_fingerprint", $PLATFORM_PREVIEW_SDK_FINGERPRINT)
		writeProp("ro.build.version.known_codenames", $PLATFORM_VERSION_KNOWN_CODENAMES)
		writeProp("ro.build.version.release_or_preview_display", $PLATFORM_DISPLAY_VERSION)
		writeProp("ro.build.date", `$DATE`)
		writeProp("ro.build.date.utc", `$DATE +%s`)
		writeProp("ro.build.user", $BUILD_USERNAME)
		writeProp("ro.build.host", $BUILD_HOSTNAME)
		writeProp("ro.build.tags", $BUILD_VERSION_TAGS)
		writeProp("ro.build.flavor", $TARGET_BUILD_FLAVOR)
		// These values are deprecated, use "ro.product.cpu.abilist"
		// instead (see below).
		writeString("# ro.product.cpu.abi and ro.product.cpu.abi2 are obsolete,")
		writeString("# use ro.product.cpu.abilist instead.")
		writeProp("ro.product.cpu.abi", $TARGET_CPU_ABI)
		if [ -n "$TARGET_CPU_ABI2" ] {
			writeProp("ro.product.cpu.abi2", $TARGET_CPU_ABI2)
		}

		if [ -n "$PRODUCT_DEFAULT_LOCALE" ] {
			writeProp("ro.product.locale", $PRODUCT_DEFAULT_LOCALE)
		}
		writeProp("ro.wifi.channels", $PRODUCT_DEFAULT_WIFI_CHANNELS)
		writeString("# ro.build.product is obsolete; use ro.product.device")
		writeProp("ro.build.product", $TARGET_DEVICE)

		writeString("# Do not try to parse description or thumbprint")
		writeProp("ro.build.description", $PRIVATE_BUILD_DESC)
		if [ -n "$BUILD_THUMBPRINT" ] {
			writeProp("ro.build.thumbprint", $BUILD_THUMBPRINT)
		}
	*/

	writeString("# end build properties")

	cmd.Text("true) > ").Output(p.outputFilePath)
	rule.Build("build.prop", "generating build.prop")

	if !p.installable() {
		p.SkipInstall()
	}

	p.installPath = PathForModuleInstall(ctx)
	ctx.InstallFile(p.installPath, p.Name(), p.outputFilePath)
}

func (f *buildinfoPropModule) GenerateSingletonBuildActions(ctx SingletonContext) {
	// does nothing; buildinfo_prop is a singeton because two buildinfo modules don't make sense.
}

func (p *buildinfoPropModule) AndroidMkEntries() []AndroidMkEntries {
	return []AndroidMkEntries{AndroidMkEntries{
		Class:      "ETC",
		OutputFile: OptionalPathForPath(p.outputFilePath),
		ExtraEntries: []AndroidMkExtraEntriesFunc{
			func(ctx AndroidMkExtraEntriesContext, entries *AndroidMkEntries) {
				entries.SetString("LOCAL_MODULE_PATH", p.installPath.String())
				entries.SetString("LOCAL_INSTALLED_MODULE_STEM", p.outputFilePath.Base())
				entries.SetBoolIfTrue("LOCAL_UNINSTALLABLE_MODULE", !p.installable())
			},
		},
	}}
}

// buildinfo_prop module generates a build.prop file, which contains a set of common
// system/build.prop properties, such as ro.build.version.*.  Not all properties are implemented;
// currently this module is only for microdroid.
func buildinfoPropFactory() SingletonModule {
	module := &buildinfoPropModule{}
	module.AddProperties(&module.properties)
	InitAndroidModule(module)
	return module
}
