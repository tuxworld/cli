package quota_test

import (
	"github.com/cloudfoundry/cli/cf"
	"github.com/cloudfoundry/cli/cf/commandregistry"
	"github.com/cloudfoundry/cli/cf/configuration/coreconfig"
	"github.com/cloudfoundry/cli/flags"
	. "github.com/cloudfoundry/cli/testhelpers/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/blang/semver"
	"github.com/cloudfoundry/cli/cf/api/quotas/quotasfakes"
	"github.com/cloudfoundry/cli/cf/api/resources"
	"github.com/cloudfoundry/cli/cf/commands/quota"
	"github.com/cloudfoundry/cli/cf/errors"
	"github.com/cloudfoundry/cli/cf/requirements"
	"github.com/cloudfoundry/cli/cf/requirements/requirementsfakes"
	testcmd "github.com/cloudfoundry/cli/testhelpers/commands"
	testconfig "github.com/cloudfoundry/cli/testhelpers/configuration"
	testreq "github.com/cloudfoundry/cli/testhelpers/requirements"
	testterm "github.com/cloudfoundry/cli/testhelpers/terminal"
)

var _ = Describe("create-quota command", func() {
	var (
		ui                  *testterm.FakeUI
		quotaRepo           *quotasfakes.FakeQuotaRepository
		requirementsFactory *testreq.FakeReqFactory
		configRepo          coreconfig.Repository
		deps                commandregistry.Dependency
	)

	updateCommandDependency := func(pluginCall bool) {
		deps.UI = ui
		deps.Config = configRepo
		deps.RepoLocator = deps.RepoLocator.SetQuotaRepository(quotaRepo)
		commandregistry.Commands.SetCommand(commandregistry.Commands.FindCommand("create-quota").SetDependency(deps, pluginCall))
	}

	BeforeEach(func() {
		ui = &testterm.FakeUI{}
		configRepo = testconfig.NewRepositoryWithDefaults()
		quotaRepo = new(quotasfakes.FakeQuotaRepository)
		requirementsFactory = &testreq.FakeReqFactory{}
	})

	runCommand := func(args ...string) bool {
		return testcmd.RunCLICommand("create-quota", args, requirementsFactory, updateCommandDependency, false)
	}

	Describe("Help text", func() {
		var usage string

		BeforeEach(func() {
			cq := &quota.CreateQuota{}
			up := commandregistry.CLICommandUsagePresenter(cq)
			usage = up.Usage()
		})

		It("has a reserved route ports flag", func() {
			Expect(usage).To(MatchRegexp(`--reserved-route-ports\s+Maximum number of routes that may be created with reserved ports \(Default: 0\)`))

			Expect(usage).To(MatchRegexp(`cf create-quota.*\[--reserved-route-ports\]`))
		})

		It("has an instance memory flag", func() {
			Expect(usage).To(MatchRegexp(`-i\s+Maximum amount of memory an application instance can have \(e.g. 1024M, 1G, 10G\). -1 represents an unlimited amount.`))

			Expect(usage).To(MatchRegexp(`cf create-quota.*\[-i INSTANCE_MEMORY\]`))
		})

		It("has a total memory flag", func() {
			Expect(usage).To(MatchRegexp(`-m\s+Total amount of memory \(e.g. 1024M, 1G, 10G\)`))

			Expect(usage).To(MatchRegexp(`cf create-quota.*\[-m TOTAL_MEMORY\]`))
		})

		It("has a routes flag", func() {
			Expect(usage).To(MatchRegexp(`-r\s+Total number of routes`))

			Expect(usage).To(MatchRegexp(`cf create-quota.*\[-r ROUTES\]`))
		})

		It("has a service instances flag", func() {
			Expect(usage).To(MatchRegexp(`-s\s+Total number of service instances`))

			Expect(usage).To(MatchRegexp(`cf create-quota.*\[-s SERVICE_INSTANCES\]`))
		})

		It("has an app instances flag", func() {
			Expect(usage).To(MatchRegexp(`-a\s+Total number of application instances. -1 represents an unlimited amount. \(Default: unlimited\)`))

			Expect(usage).To(MatchRegexp(`cf create-quota.*\[-a APP_INSTANCES\]`))
		})
	})

	Context("when the user is not logged in", func() {
		BeforeEach(func() {
			requirementsFactory.LoginSuccess = false
		})

		It("fails requirements", func() {
			Expect(runCommand("my-quota", "-m", "50G")).To(BeFalse())
		})
	})

	Context("the minimum API version requirement", func() {
		BeforeEach(func() {
			requirementsFactory.LoginSuccess = true
			requirementsFactory.MinAPIVersionSuccess = false
		})

		It("fails when the -a option is provided", func() {
			Expect(runCommand("my-quota", "-a", "10")).To(BeFalse())

			Expect(requirementsFactory.MinAPIVersionRequiredVersion).To(Equal(cf.OrgAppInstanceLimitMinimumAPIVersion))
			Expect(requirementsFactory.MinAPIVersionFeatureName).To(Equal("Option '-a'"))
		})

		It("does not fail when the -a option is not provided", func() {
			Expect(runCommand("my-quota", "-m", "10G")).To(BeTrue())
		})
	})

	Context("when the user is logged in", func() {
		BeforeEach(func() {
			requirementsFactory.LoginSuccess = true
			requirementsFactory.MinAPIVersionSuccess = true
		})

		It("fails requirements when called without a quota name", func() {
			runCommand()
			Expect(ui.Outputs).To(ContainSubstrings(
				[]string{"Incorrect Usage", "Requires an argument"},
			))
		})

		It("creates a quota with a given name", func() {
			runCommand("my-quota")
			Expect(quotaRepo.CreateArgsForCall(0).Name).To(Equal("my-quota"))
			Expect(ui.Outputs).To(ContainSubstrings(
				[]string{"Creating quota", "my-quota", "my-user", "..."},
				[]string{"OK"},
			))
		})

		Context("when the -i flag is not provided", func() {
			It("defaults the memory limit to unlimited", func() {
				runCommand("my-quota")

				Expect(quotaRepo.CreateArgsForCall(0).InstanceMemoryLimit).To(Equal(int64(-1)))
			})
		})

		Context("when the -m flag is provided", func() {
			It("sets the memory limit", func() {
				runCommand("-m", "50G", "erryday makin fitty jeez")
				Expect(quotaRepo.CreateArgsForCall(0).MemoryLimit).To(Equal(int64(51200)))
			})

			It("alerts the user when parsing the memory limit fails", func() {
				runCommand("whoops", "12")

				Expect(ui.Outputs).To(ContainSubstrings([]string{"FAILED"}))
			})
		})

		Context("when the -i flag is provided", func() {
			It("sets the memory limit", func() {
				runCommand("-i", "50G", "erryday makin fitty jeez")
				Expect(quotaRepo.CreateArgsForCall(0).InstanceMemoryLimit).To(Equal(int64(51200)))
			})

			It("alerts the user when parsing the memory limit fails", func() {
				runCommand("-i", "whoops", "wit mah hussle", "12")

				Expect(ui.Outputs).To(ContainSubstrings([]string{"FAILED"}))
			})

			Context("and the provided value is -1", func() {
				It("sets the memory limit", func() {
					runCommand("-i", "-1", "yo")
					Expect(quotaRepo.CreateArgsForCall(0).InstanceMemoryLimit).To(Equal(int64(-1)))
				})
			})
		})

		Context("when the -a flag is provided", func() {
			It("sets the app limit", func() {
				runCommand("my-quota", "-a", "10")

				Expect(quotaRepo.CreateArgsForCall(0).AppInstanceLimit).To(Equal(10))
			})

			It("defaults to unlimited", func() {
				runCommand("my-quota")

				Expect(quotaRepo.CreateArgsForCall(0).AppInstanceLimit).To(Equal(resources.UnlimitedAppInstances))
			})
		})

		It("sets the route limit", func() {
			runCommand("-r", "12", "ecstatic")

			Expect(quotaRepo.CreateArgsForCall(0).RoutesLimit).To(Equal(12))
		})

		It("sets the service instance limit", func() {
			runCommand("-s", "42", "black star")
			Expect(quotaRepo.CreateArgsForCall(0).ServicesLimit).To(Equal(42))
		})

		Context("when requesting to allow paid service plans", func() {
			It("creates the quota with paid service plans allowed", func() {
				runCommand("--allow-paid-service-plans", "my-for-profit-quota")
				Expect(quotaRepo.CreateArgsForCall(0).NonBasicServicesAllowed).To(BeTrue())
			})

			It("defaults to not allowing paid service plans", func() {
				runCommand("my-pro-bono-quota")
				Expect(quotaRepo.CreateArgsForCall(0).NonBasicServicesAllowed).To(BeFalse())
			})
		})

		Context("when creating a quota returns an error", func() {
			It("alerts the user when creating the quota fails", func() {
				quotaRepo.CreateReturns(errors.New("WHOOP THERE IT IS"))
				runCommand("my-quota")

				Expect(ui.Outputs).To(ContainSubstrings(
					[]string{"Creating quota", "my-quota"},
					[]string{"FAILED"},
				))
			})

			It("warns the user when quota already exists", func() {
				quotaRepo.CreateReturns(errors.NewHTTPError(400, errors.QuotaDefinitionNameTaken, "Quota Definition is taken: quota-sct"))
				runCommand("Banana")

				Expect(ui.Outputs).ToNot(ContainSubstrings(
					[]string{"FAILED"},
				))
				Expect(ui.WarnOutputs).To(ContainSubstrings([]string{"already exists"}))
			})

		})
	})

	Describe("Requirements", func() {
		var (
			requirementsFactory *requirementsfakes.FakeFactory

			ui   *testterm.FakeUI
			cmd  commandregistry.Command
			deps commandregistry.Dependency

			quotaRepo   *quotasfakes.FakeQuotaRepository
			flagContext flags.FlagContext

			loginRequirement         requirements.Requirement
			minAPIVersionRequirement requirements.Requirement
		)

		BeforeEach(func() {
			ui = &testterm.FakeUI{}

			configRepo = testconfig.NewRepositoryWithDefaults()
			quotaRepo = new(quotasfakes.FakeQuotaRepository)
			repoLocator := deps.RepoLocator.SetQuotaRepository(quotaRepo)

			deps = commandregistry.Dependency{
				UI:          ui,
				Config:      configRepo,
				RepoLocator: repoLocator,
			}

			requirementsFactory = new(requirementsfakes.FakeFactory)

			cmd = &quota.CreateQuota{}
			cmd.SetDependency(deps, false)

			flagContext = flags.NewFlagContext(cmd.MetaData().Flags)

			loginRequirement = &passingRequirement{Name: "login-requirement"}
			requirementsFactory.NewLoginRequirementReturns(loginRequirement)

			minAPIVersionRequirement = &passingRequirement{Name: "min-api-version-requirement"}
			requirementsFactory.NewMinAPIVersionRequirementReturns(minAPIVersionRequirement)
		})

		Context("when not provided exactly one arg", func() {
			BeforeEach(func() {
				flagContext.Parse("quota", "extra-arg")
			})

			It("fails with usage", func() {
				Expect(func() { cmd.Requirements(requirementsFactory, flagContext) }).To(Panic())
				Expect(ui.Outputs).To(ContainSubstrings(
					[]string{"FAILED"},
					[]string{"Incorrect Usage. Requires an argument"},
				))
			})
		})

		Context("when provided exactly one arg", func() {
			BeforeEach(func() {
				flagContext.Parse("quota")
			})

			It("returns a LoginRequirement", func() {
				actualRequirements := cmd.Requirements(requirementsFactory, flagContext)
				Expect(requirementsFactory.NewLoginRequirementCallCount()).To(Equal(1))
				Expect(actualRequirements).To(ContainElement(loginRequirement))
			})

			It("does not return a MinAPIVersionRequirement", func() {
				actualRequirements := cmd.Requirements(requirementsFactory, flagContext)
				Expect(requirementsFactory.NewMinAPIVersionRequirementCallCount()).To(Equal(0))
				Expect(actualRequirements).NotTo(ContainElement(minAPIVersionRequirement))
			})

			Context("when an app instance limit is passed", func() {
				BeforeEach(func() {
					flagContext = flags.NewFlagContext(cmd.MetaData().Flags)
					flagContext.Parse("domain-name", "-a", "2")
				})

				It("returns a MinAPIVersionRequirement as the second requirement", func() {
					actualRequirements := cmd.Requirements(requirementsFactory, flagContext)

					expectedVersion, err := semver.Make("2.33.0")
					Expect(err).NotTo(HaveOccurred())

					Expect(requirementsFactory.NewMinAPIVersionRequirementCallCount()).To(Equal(1))
					feature, requiredVersion := requirementsFactory.NewMinAPIVersionRequirementArgsForCall(0)
					Expect(feature).To(Equal("Option '-a'"))
					Expect(requiredVersion).To(Equal(expectedVersion))
					Expect(actualRequirements[1]).To(Equal(minAPIVersionRequirement))
				})
			})

			Context("when reserved route ports limit is passed", func() {
				BeforeEach(func() {
					flagContext = flags.NewFlagContext(cmd.MetaData().Flags)
					flagContext.Parse("domain-name", "--reserved-route-ports", "3")
				})

				It("returns a MinAPIVersionRequirement as the second requirement", func() {
					actualRequirements := cmd.Requirements(requirementsFactory, flagContext)

					expectedVersion, err := semver.Make("2.55.0")
					Expect(err).NotTo(HaveOccurred())

					Expect(requirementsFactory.NewMinAPIVersionRequirementCallCount()).To(Equal(1))
					feature, requiredVersion := requirementsFactory.NewMinAPIVersionRequirementArgsForCall(0)
					Expect(feature).To(Equal("Option '--reserved-route-ports'"))
					Expect(requiredVersion).To(Equal(expectedVersion))
					Expect(actualRequirements[1]).To(Equal(minAPIVersionRequirement))
				})
			})
		})
	})
})

type passingRequirement struct {
	Name string
}

func (r passingRequirement) Execute() error {
	return nil
}