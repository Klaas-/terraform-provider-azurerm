package appservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/web/mgmt/2021-02-01/web"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/azure"
	"github.com/hashicorp/terraform-provider-azurerm/internal/location"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/appservice/helpers"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/appservice/parse"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/appservice/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tags"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

type WindowsWebAppResource struct{}

type WindowsWebAppModel struct {
	Name                          string                      `tfschema:"name"`
	ResourceGroup                 string                      `tfschema:"resource_group_name"`
	Location                      string                      `tfschema:"location"`
	ServicePlanId                 string                      `tfschema:"service_plan_id"`
	AppSettings                   map[string]string           `tfschema:"app_settings"`
	AuthSettings                  []helpers.AuthSettings      `tfschema:"auth_settings"`
	Backup                        []helpers.Backup            `tfschema:"backup"`
	ClientAffinityEnabled         bool                        `tfschema:"client_affinity_enabled"`
	ClientCertEnabled             bool                        `tfschema:"client_cert_enabled"`
	ClientCertMode                string                      `tfschema:"client_cert_mode"`
	Enabled                       bool                        `tfschema:"enabled"`
	HttpsOnly                     bool                        `tfschema:"https_only"`
	Identity                      []helpers.Identity          `tfschema:"identity"`
	LogsConfig                    []helpers.LogsConfig        `tfschema:"logs"`
	SiteConfig                    []helpers.SiteConfigWindows `tfschema:"site_config"`
	StorageAccounts               []helpers.StorageAccount    `tfschema:"storage_account"`
	ConnectionStrings             []helpers.ConnectionString  `tfschema:"connection_string"`
	CustomDomainVerificationId    string                      `tfschema:"custom_domain_verification_id"`
	DefaultHostname               string                      `tfschema:"default_hostname"`
	Kind                          string                      `tfschema:"kind"`
	OutboundIPAddresses           string                      `tfschema:"outbound_ip_addresses"`
	OutboundIPAddressList         []string                    `tfschema:"outbound_ip_address_list"`
	PossibleOutboundIPAddresses   string                      `tfschema:"possible_outbound_ip_addresses"`
	PossibleOutboundIPAddressList []string                    `tfschema:"possible_outbound_ip_address_list"`
	SiteCredentials               []helpers.SiteCredential    `tfschema:"site_credential"`
	Tags                          map[string]string           `tfschema:"tags"`
}

var _ sdk.Resource = WindowsWebAppResource{}
var _ sdk.ResourceWithUpdate = WindowsWebAppResource{}

// TODO - Feature: Deployments (Preview)?
// TODO - Feature: App Insights?

func (r WindowsWebAppResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ForceNew:     true,
			ValidateFunc: validate.WebAppName,
		},

		"resource_group_name": azure.SchemaResourceGroupName(),

		"location": location.Schema(),

		"service_plan_id": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ValidateFunc: validate.ServicePlanID,
		},

		// Optional

		"app_settings": {
			Type:     pluginsdk.TypeMap,
			Optional: true,
			Computed: true,
			Elem: &pluginsdk.Schema{
				Type: pluginsdk.TypeString,
			},
		},

		"auth_settings": helpers.AuthSettingsSchema(),

		"backup": helpers.BackupSchema(),

		"client_affinity_enabled": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			Default:  false,
		},

		"client_cert_enabled": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			Default:  false,
		},

		"client_cert_mode": {
			Type:     pluginsdk.TypeString,
			Optional: true,
			Default:  "Required",
			ValidateFunc: validation.StringInSlice([]string{
				string(web.ClientCertModeOptional),
				string(web.ClientCertModeRequired),
			}, false),
		},

		"connection_string": helpers.ConnectionStringSchema(),

		"enabled": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			Default:  true,
		},

		"https_only": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			Default:  false,
		},

		"identity": helpers.IdentitySchema(),

		"logs": helpers.LogsConfigSchema(),

		"site_config": helpers.SiteConfigSchemaWindows(),

		"storage_account": helpers.StorageAccountSchema(),

		"tags": tags.Schema(),
	}
}

func (r WindowsWebAppResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"custom_domain_verification_id": {
			Type:      pluginsdk.TypeString,
			Computed:  true,
			Sensitive: true,
		},

		"default_hostname": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},

		"kind": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},

		"outbound_ip_addresses": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},

		"outbound_ip_address_list": {
			Type:     pluginsdk.TypeList,
			Computed: true,
			Elem: &pluginsdk.Schema{
				Type: pluginsdk.TypeString,
			},
		},

		"possible_outbound_ip_addresses": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},

		"possible_outbound_ip_address_list": {
			Type:     pluginsdk.TypeList,
			Computed: true,
			Elem: &pluginsdk.Schema{
				Type: pluginsdk.TypeString,
			},
		},

		"site_credential": helpers.SiteCredentialSchema(),
	}
}

func (r WindowsWebAppResource) ModelObject() interface{} {
	return WindowsWebAppModel{}
}

func (r WindowsWebAppResource) ResourceType() string {
	return "azurerm_windows_web_app"
}

func (r WindowsWebAppResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			var webApp WindowsWebAppModel
			if err := metadata.Decode(&webApp); err != nil {
				return err
			}

			client := metadata.Client.AppService.WebAppsClient
			servicePlanClient := metadata.Client.AppService.ServicePlanClient
			aseClient := metadata.Client.AppService.AppServiceEnvironmentClient
			subscriptionId := metadata.Client.Account.SubscriptionId

			id := parse.NewWebAppID(subscriptionId, webApp.ResourceGroup, webApp.Name)

			existing, err := client.Get(ctx, id.ResourceGroup, id.SiteName)
			if err != nil && !utils.ResponseWasNotFound(existing.Response) {
				return fmt.Errorf("checking for presence of existing Windows %s: %+v", id, err)
			}

			if !utils.ResponseWasNotFound(existing.Response) {
				return metadata.ResourceRequiresImport(r.ResourceType(), id)
			}

			availabilityRequest := web.ResourceNameAvailabilityRequest{
				Name: utils.String(webApp.Name),
				Type: web.CheckNameResourceTypesMicrosoftWebsites,
			}

			servicePlanId, err := parse.ServicePlanID(webApp.ServicePlanId)
			if err != nil {
				return err
			}

			servicePlan, err := servicePlanClient.Get(ctx, servicePlanId.ResourceGroup, servicePlanId.ServerfarmName)
			if err != nil {
				return fmt.Errorf("reading App %s: %+v", servicePlanId, err)
			}
			if ase := servicePlan.HostingEnvironmentProfile; ase != nil {
				// Attempt to check the ASE for the appropriate suffix for the name availability request. Not convinced
				// the `DNSSuffix` field is still valid and possibly should have been deprecated / removed as is legacy
				// setting from ASEv1? Hence the non-fatal approach here.
				nameSuffix := "appserviceenvironment.net"
				if ase.ID != nil {
					aseId, err := parse.AppServiceEnvironmentID(*ase.ID)
					nameSuffix = fmt.Sprintf("%s.%s", aseId.HostingEnvironmentName, nameSuffix)
					if err != nil {
						metadata.Logger.Warnf("could not parse App Service Environment ID determine FQDN for name availability check, defaulting to `%s.%s.appserviceenvironment.net`", webApp.Name, servicePlanId)
					} else {
						existingASE, err := aseClient.Get(ctx, aseId.ResourceGroup, aseId.HostingEnvironmentName)
						if err != nil {
							metadata.Logger.Warnf("could not read App Service Environment to determine FQDN for name availability check, defaulting to `%s.%s.appserviceenvironment.net`", webApp.Name, servicePlanId)
						} else if props := existingASE.AppServiceEnvironment; props != nil && props.DNSSuffix != nil && *props.DNSSuffix != "" {
							nameSuffix = *props.DNSSuffix
						}
					}
				}
				availabilityRequest.Name = utils.String(fmt.Sprintf("%s.%s.%s", webApp.Name, servicePlanId.ServerfarmName, nameSuffix))
				availabilityRequest.IsFqdn = utils.Bool(true)
			}

			checkName, err := client.CheckNameAvailability(ctx, availabilityRequest)
			if err != nil {
				return fmt.Errorf("checking name availability for %s: %+v", id, err)
			}
			if !*checkName.NameAvailable {
				return fmt.Errorf("the Site Name %q failed the availability check: %+v", id.SiteName, *checkName.Message)
			}

			siteConfig, currentStack, err := helpers.ExpandSiteConfigWindows(webApp.SiteConfig)
			if err != nil {
				return err
			}

			siteEnvelope := web.Site{
				Location: utils.String(webApp.Location),
				Tags:     tags.FromTypedObject(webApp.Tags),
				SiteProperties: &web.SiteProperties{
					ServerFarmID:          utils.String(webApp.ServicePlanId),
					Enabled:               utils.Bool(webApp.Enabled),
					HTTPSOnly:             utils.Bool(webApp.HttpsOnly),
					SiteConfig:            siteConfig,
					ClientAffinityEnabled: utils.Bool(webApp.ClientAffinityEnabled),
					ClientCertEnabled:     utils.Bool(webApp.ClientCertEnabled),
					ClientCertMode:        web.ClientCertMode(webApp.ClientCertMode),
				},
			}

			if identity := helpers.ExpandIdentity(webApp.Identity); identity != nil {
				siteEnvelope.Identity = identity
			}

			future, err := client.CreateOrUpdate(ctx, id.ResourceGroup, id.SiteName, siteEnvelope)
			if err != nil {
				return fmt.Errorf("creating Windows %s: %+v", id, err)
			}

			if err := future.WaitForCompletionRef(ctx, client.Client); err != nil {
				return fmt.Errorf("waiting for creation of Windows %s: %+v", id, err)
			}

			if currentStack != nil && *currentStack != "" {
				siteMetadata := web.StringDictionary{Properties: map[string]*string{}}
				siteMetadata.Properties["CURRENT_STACK"] = currentStack
				if _, err := client.UpdateMetadata(ctx, id.ResourceGroup, id.SiteName, siteMetadata); err != nil {
					return fmt.Errorf("setting Site Metadata for Current Stack on Windows %s: %+v", id, err)
				}
			}

			metadata.SetID(id)

			appSettings := helpers.ExpandAppSettings(webApp.AppSettings)
			if appSettings != nil {
				if _, err := client.UpdateApplicationSettings(ctx, id.ResourceGroup, id.SiteName, *appSettings); err != nil {
					return fmt.Errorf("setting App Settings for Windows %s: %+v", id, err)
				}
			}

			auth := helpers.ExpandAuthSettings(webApp.AuthSettings)
			if auth.SiteAuthSettingsProperties != nil {
				if _, err := client.UpdateAuthSettings(ctx, id.ResourceGroup, id.SiteName, *auth); err != nil {
					return fmt.Errorf("setting Authorisation Settings for %s: %+v", id, err)
				}
			}

			logsConfig := helpers.ExpandLogsConfig(webApp.LogsConfig)
			if logsConfig.SiteLogsConfigProperties != nil {
				if _, err := client.UpdateDiagnosticLogsConfig(ctx, id.ResourceGroup, id.SiteName, *logsConfig); err != nil {
					return fmt.Errorf("setting Diagnostic Logs Configuration for Windows %s: %+v", id, err)
				}
			}

			backupConfig := helpers.ExpandBackupConfig(webApp.Backup)
			if backupConfig.BackupRequestProperties != nil {
				if _, err := client.UpdateBackupConfiguration(ctx, id.ResourceGroup, id.SiteName, *backupConfig); err != nil {
					return fmt.Errorf("adding Backup Settings for Windows %s: %+v", id, err)
				}
			}

			storageConfig := helpers.ExpandStorageConfig(webApp.StorageAccounts)
			if storageConfig.Properties != nil {
				if _, err := client.UpdateAzureStorageAccounts(ctx, id.ResourceGroup, id.SiteName, *storageConfig); err != nil {
					if err != nil {
						return fmt.Errorf("setting Storage Accounts for Windows %s: %+v", id, err)
					}
				}
			}

			connectionStrings := helpers.ExpandConnectionStrings(webApp.ConnectionStrings)
			if connectionStrings.Properties != nil {
				if _, err := client.UpdateConnectionStrings(ctx, id.ResourceGroup, id.SiteName, *connectionStrings); err != nil {
					return fmt.Errorf("setting Connection Strings for Windows %s: %+v", id, err)
				}
			}

			return nil
		},

		Timeout: 30 * time.Minute,
	}
}

func (r WindowsWebAppResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.AppService.WebAppsClient
			id, err := parse.WebAppID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}
			webApp, err := client.Get(ctx, id.ResourceGroup, id.SiteName)
			if err != nil {
				if utils.ResponseWasNotFound(webApp.Response) {
					return metadata.MarkAsGone(id)
				}
				return fmt.Errorf("reading Windows %s: %+v", id, err)
			}

			if webApp.SiteProperties == nil {
				return fmt.Errorf("reading properties of Windows %s", id)
			}

			// Despite being part of the defined `Get` response model, site_config is always nil so we get it explicitly
			webAppSiteConfig, err := client.GetConfiguration(ctx, id.ResourceGroup, id.SiteName)
			if err != nil {
				return fmt.Errorf("reading Site Config for Windows %s: %+v", id, err)
			}

			auth, err := client.GetAuthSettings(ctx, id.ResourceGroup, id.SiteName)
			if err != nil {
				return fmt.Errorf("reading Auth Settings for Windows %s: %+v", id, err)
			}

			backup, err := client.GetBackupConfiguration(ctx, id.ResourceGroup, id.SiteName)
			if err != nil {
				if !utils.ResponseWasNotFound(backup.Response) {
					return fmt.Errorf("reading Backup Settings for Windows %s: %+v", id, err)
				}
			}

			logsConfig, err := client.GetDiagnosticLogsConfiguration(ctx, id.ResourceGroup, id.SiteName)
			if err != nil {
				return fmt.Errorf("reading Diagnostic Logs information for Windows %s: %+v", id, err)
			}

			appSettings, err := client.ListApplicationSettings(ctx, id.ResourceGroup, id.SiteName)
			if err != nil {
				return fmt.Errorf("reading App Settings for Windows %s: %+v", id, err)
			}

			storageAccounts, err := client.ListAzureStorageAccounts(ctx, id.ResourceGroup, id.SiteName)
			if err != nil {
				return fmt.Errorf("reading Storage Account information for Windows %s: %+v", id, err)
			}

			connectionStrings, err := client.ListConnectionStrings(ctx, id.ResourceGroup, id.SiteName)
			if err != nil {
				return fmt.Errorf("reading Connection String information for Windows %s: %+v", id, err)
			}

			siteCredentialsFuture, err := client.ListPublishingCredentials(ctx, id.ResourceGroup, id.SiteName)
			if err != nil {
				return fmt.Errorf("listing Site Publishing Credential information for Windows %s: %+v", id, err)
			}

			if err := siteCredentialsFuture.WaitForCompletionRef(ctx, client.Client); err != nil {
				return fmt.Errorf("waiting for Site Publishing Credential information for Windows %s: %+v", id, err)
			}
			siteCredentials, err := siteCredentialsFuture.Result(*client)
			if err != nil {
				return fmt.Errorf("reading Site Publishing Credential information for Windows %s: %+v", id, err)
			}

			siteMetadata, err := client.ListMetadata(ctx, id.ResourceGroup, id.SiteName)
			if err != nil {
				return fmt.Errorf("reading Site Metadata for Windows %s: %+v", id, err)
			}

			state := WindowsWebAppModel{
				Name:          id.SiteName,
				ResourceGroup: id.ResourceGroup,
				Location:      location.NormalizeNilable(webApp.Location),
				AppSettings:   helpers.FlattenAppSettings(appSettings),
				Tags:          tags.ToTypedObject(webApp.Tags),
			}

			webAppProps := webApp.SiteProperties
			if v := webAppProps.ServerFarmID; v != nil {
				state.ServicePlanId = *v
			}

			if v := webAppProps.ClientAffinityEnabled; v != nil {
				state.ClientAffinityEnabled = *v
			}

			if v := webAppProps.ClientCertEnabled; v != nil {
				state.ClientCertEnabled = *v
			}

			if webAppProps.ClientCertMode != "" {
				state.ClientCertMode = string(webAppProps.ClientCertMode)
			}

			if v := webAppProps.Enabled; v != nil {
				state.Enabled = *v
			}

			if v := webAppProps.HTTPSOnly; v != nil {
				state.HttpsOnly = *v
			}

			if v := webAppProps.CustomDomainVerificationID; v != nil {
				state.CustomDomainVerificationId = *v
			}

			if v := webAppProps.DefaultHostName; v != nil {
				state.DefaultHostname = *v
			}

			if v := webApp.Kind; v != nil {
				state.Kind = *v
			}

			if v := webAppProps.OutboundIPAddresses; v != nil {
				state.OutboundIPAddresses = *v
				state.OutboundIPAddressList = strings.Split(*v, ",")
			}

			if v := webAppProps.PossibleOutboundIPAddresses; v != nil {
				state.PossibleOutboundIPAddresses = *v
				state.PossibleOutboundIPAddressList = strings.Split(*v, ",")
			}

			if appAuthSettings := helpers.FlattenAuthSettings(auth); appAuthSettings != nil {
				state.AuthSettings = appAuthSettings
			}

			if appBackupSettings := helpers.FlattenBackupConfig(backup); appBackupSettings != nil {
				state.Backup = appBackupSettings
			}

			if identity := helpers.FlattenIdentity(webApp.Identity); identity != nil {
				state.Identity = identity
			}

			if logs := helpers.FlattenLogsConfig(logsConfig); logs != nil {
				state.LogsConfig = logs
			}

			currentStack := ""
			currentStackPtr, ok := siteMetadata.Properties["CURRENT_STACK"]
			if ok {
				currentStack = *currentStackPtr
			}
			if siteConfig := helpers.FlattenSiteConfigWindows(webAppSiteConfig.SiteConfig, currentStack); siteConfig != nil {
				state.SiteConfig = siteConfig
			}

			if appStorageAccounts := helpers.FlattenStorageAccounts(storageAccounts); appStorageAccounts != nil {
				state.StorageAccounts = appStorageAccounts
			}

			if appConnectionStrings := helpers.FlattenConnectionStrings(connectionStrings); appConnectionStrings != nil {
				state.ConnectionStrings = appConnectionStrings
			}

			state.SiteCredentials = helpers.FlattenSiteCredentials(siteCredentials)

			return metadata.Encode(&state)
		},
	}
}

func (r WindowsWebAppResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.AppService.WebAppsClient
			id, err := parse.WebAppID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			metadata.Logger.Infof("deleting %s", *id)

			deleteMetrics := true // TODO - Look at making this a feature flag?
			deleteEmptyServerFarm := false
			if _, err := client.Delete(ctx, id.ResourceGroup, id.SiteName, &deleteMetrics, &deleteEmptyServerFarm); err != nil {
				return fmt.Errorf("deleting Windows %s: %+v", id, err)
			}
			return nil
		},
	}
}

func (r WindowsWebAppResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return validate.WebAppID
}

func (r WindowsWebAppResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.AppService.WebAppsClient

			id, err := parse.WebAppID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			// TODO - Need locking here when the source control meta resource is added

			var state WindowsWebAppModel
			if err := metadata.Decode(&state); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			site := web.Site{
				Location: utils.String(state.Location),
				Tags:     tags.FromTypedObject(state.Tags),
				SiteProperties: &web.SiteProperties{
					ServerFarmID:          utils.String(state.ServicePlanId),
					Enabled:               utils.Bool(state.Enabled),
					HTTPSOnly:             utils.Bool(state.HttpsOnly),
					ClientAffinityEnabled: utils.Bool(state.ClientAffinityEnabled),
					ClientCertEnabled:     utils.Bool(state.ClientCertEnabled),
					ClientCertMode:        web.ClientCertMode(state.ClientCertMode),
				},
				Identity: helpers.ExpandIdentity(state.Identity),
			}

			siteConfig, currentStack, err := helpers.ExpandSiteConfigWindows(state.SiteConfig)
			if err != nil {
				return fmt.Errorf("expanding Site Config for Windows %s: %+v", id, err)
			}

			site.SiteConfig = siteConfig
			updateFuture, err := client.CreateOrUpdate(ctx, id.ResourceGroup, id.SiteName, site)
			if err != nil {
				return fmt.Errorf("updating Windows %s: %+v", id, err)
			}
			if err := updateFuture.WaitForCompletionRef(ctx, client.Client); err != nil {
				return fmt.Errorf("wating to update %s: %+v", id, err)
			}

			if currentStack != nil && *currentStack != "" {
				siteMetadata := web.StringDictionary{Properties: map[string]*string{}}
				siteMetadata.Properties["CURRENT_STACK"] = currentStack
				if _, err := client.UpdateMetadata(ctx, id.ResourceGroup, id.SiteName, siteMetadata); err != nil {
					return fmt.Errorf("setting Site Metadata for Current Stack on Windows %s: %+v", id, err)
				}
			}

			// (@jackofallops) - App Settings can clobber logs configuration so must be updated before we send any Log updates
			if metadata.ResourceData.HasChange("app_settings") {
				appSettingsUpdate := helpers.ExpandAppSettings(state.AppSettings)
				if _, err := client.UpdateApplicationSettings(ctx, id.ResourceGroup, id.SiteName, *appSettingsUpdate); err != nil {
					return fmt.Errorf("updating App Settings for Windows %s: %+v", id, err)
				}
			}

			if metadata.ResourceData.HasChange("connection_string") {
				connectionStringUpdate := helpers.ExpandConnectionStrings(state.ConnectionStrings)
				if _, err := client.UpdateConnectionStrings(ctx, id.ResourceGroup, id.SiteName, *connectionStringUpdate); err != nil {
					return fmt.Errorf("updating Connection Strings for Windows %s: %+v", id, err)
				}
			}

			if metadata.ResourceData.HasChange("auth_settings") {
				authUpdate := helpers.ExpandAuthSettings(state.AuthSettings)
				if _, err := client.UpdateAuthSettings(ctx, id.ResourceGroup, id.SiteName, *authUpdate); err != nil {
					return fmt.Errorf("updating Auth Settings for Windows %s: %+v", id, err)
				}
			}

			if metadata.ResourceData.HasChange("backup") {
				backupUpdate := helpers.ExpandBackupConfig(state.Backup)
				if backupUpdate.BackupRequestProperties == nil {
					if _, err := client.DeleteBackupConfiguration(ctx, id.ResourceGroup, id.SiteName); err != nil {
						return fmt.Errorf("removing Backup Settings for Windows %s: %+v", id, err)
					}
				} else {
					if _, err := client.UpdateBackupConfiguration(ctx, id.ResourceGroup, id.SiteName, *backupUpdate); err != nil {
						return fmt.Errorf("updating Backup Settings for Windows %s: %+v", id, err)
					}
				}
			}

			if metadata.ResourceData.HasChange("logs") {
				logsUpdate := helpers.ExpandLogsConfig(state.LogsConfig)
				if _, err := client.UpdateDiagnosticLogsConfig(ctx, id.ResourceGroup, id.SiteName, *logsUpdate); err != nil {
					return fmt.Errorf("updating Logs Config for Windows %s: %+v", id, err)
				}
			}

			if metadata.ResourceData.HasChange("storage_account") {
				storageAccountUpdate := helpers.ExpandStorageConfig(state.StorageAccounts)
				if _, err := client.UpdateAzureStorageAccounts(ctx, id.ResourceGroup, id.SiteName, *storageAccountUpdate); err != nil {
					return fmt.Errorf("updating Storage Accounts for Windows %s: %+v", id, err)
				}
			}

			return nil
		},
	}
}
