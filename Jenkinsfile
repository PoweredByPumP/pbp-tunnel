@Library('pbpipelines') _

String projectVersion = "0.0.8-SNAPSHOT"

getIDGoPipelineV2(projectName: "pbp-tunnel",
				  projectVersion: projectVersion,
				  goVersion: "1.23.0",
				  goTarget: "./cmd/pbp-tunnel",
				  goTest: true,
				  goBuildOpts: "-ldflags=\"-X main.Version=${projectVersion}\"",
				  slaveAgent: "slave-01",
				  deploySlaveAgent: "slave-01",
				  githubAdditionallyPush: true,
				  buildForLinux: true,
				  buildForLinuxArm64: true,
				  buildForWindows: true,
				  buildForWindowsArm64: true)
