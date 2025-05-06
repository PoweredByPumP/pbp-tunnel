@Library('pbpipelines') _

getIDGoPipelineV2(projectName: "pbp-tunnel",
				  projectVersion: "0.0.7-SNAPSHOT",
				  goVersion: "1.23.0",
				  goImageOs: "debian",
				  goImage: "golang:#GO_VERSION#-bookworm",
				  goTarget: "./cmd/pbp-tunnel",
				  slaveAgent: "slave-01",
				  deploySlaveAgent: "slave-01",
				  githubAdditionallyPush: true,
				  buildForLinux: true,
				  buildForLinuxArm64: true,
				  buildForWindows: true,
				  buildForWindowsArm64: true)
