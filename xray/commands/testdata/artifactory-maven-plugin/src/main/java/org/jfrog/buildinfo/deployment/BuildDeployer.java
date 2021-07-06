package org.jfrog.buildinfo.deployment;

import org.apache.commons.collections4.ListUtils;
import org.apache.commons.collections4.MapUtils;
import org.apache.maven.plugin.logging.Log;
import org.jfrog.build.api.Artifact;
import org.jfrog.build.api.Build;
import org.jfrog.build.api.Module;
import org.jfrog.build.extractor.BuildInfoExtractorUtils;
import org.jfrog.build.extractor.ModuleParallelDeployHelper;
import org.jfrog.build.extractor.clientConfiguration.ArtifactoryClientConfiguration;
import org.jfrog.build.extractor.clientConfiguration.ArtifactoryManagerBuilder;
import org.jfrog.build.extractor.clientConfiguration.client.artifactory.ArtifactoryManager;
import org.jfrog.build.extractor.clientConfiguration.deploy.DeployDetails;
import org.jfrog.build.extractor.retention.Utils;
import org.jfrog.buildinfo.utils.ArtifactoryMavenLogger;

import java.io.IOException;
import java.util.*;

import static org.jfrog.buildinfo.utils.Utils.setChecksums;

/**
 * Deploy artifacts and publish build info.
 *
 * @author yahavi
 */
public class BuildDeployer {

    private static final String LOG_PREFIX = "Artifactory Build info recorder: ";
    private final Log logger;

    public BuildDeployer(Log logger) {
        this.logger = logger;
    }

    /**
     * Deploy artifacts and publish the build info.
     *
     * @param build               - The build to deploy
     * @param clientConf          - Artifactory client configuration
     * @param deployableArtifacts - The deployable artifacts
     */
    public void deploy(Build build, ArtifactoryClientConfiguration clientConf, Map<String, DeployDetails> deployableArtifacts) {
        Map<String, Set<DeployDetails>> deployableArtifactsByModule = prepareDeployableArtifacts(build.getModules(), deployableArtifacts);
        boolean isDeployArtifacts = isDeployArtifacts(clientConf, deployableArtifactsByModule);
        boolean isPublishBuildInfo = isPublishBuildInfo(clientConf);
        if (!isDeployArtifacts && !isPublishBuildInfo) {
            // Nothing to deploy
            return;
        }

        try (ArtifactoryManager client = new ArtifactoryManagerBuilder()
                .setClientConfiguration(clientConf, clientConf.publisher).setLog(new ArtifactoryMavenLogger(logger)).build()) {
            if (clientConf.getInsecureTls()) {
                client.setInsecureTls(true);
            }
            if (isDeployArtifacts) {
                logger.debug(LOG_PREFIX + "Publication fork count: " + clientConf.publisher.getPublishForkCount());
                new ModuleParallelDeployHelper().deployArtifacts(client, deployableArtifactsByModule, clientConf.publisher.getPublishForkCount());
            }
            if (isPublishBuildInfo) {
                logger.info(LOG_PREFIX + "Deploying build info ...");
                Utils.sendBuildAndBuildRetention(client, build, clientConf);
            }
        } catch (IOException e) {
            throw new RuntimeException(LOG_PREFIX + "Failed to deploy build info.", e);
        }
    }

    /**
     * Return true if should deploy artifacts.
     *
     * @param clientConf          - Artifactory client configuration
     * @param deployableArtifacts - The deployable artifacts
     * @return true if should deploy artifacts
     */
    private boolean isDeployArtifacts(ArtifactoryClientConfiguration clientConf, Map<String, Set<DeployDetails>> deployableArtifacts) {
        if (!clientConf.publisher.isPublishArtifacts()) {
            logger.info(LOG_PREFIX + "deploy artifacts set to false, artifacts will not be deployed...");
            return false;
        }
        if (MapUtils.isEmpty(deployableArtifacts)) {
            logger.info(LOG_PREFIX + "no artifacts to deploy...");
            return false;
        }
        return true;
    }

    /**
     * Return true if should publish build info.
     *
     * @param clientConf - Artifactory client configuration
     * @return true if should publish build info
     */
    private boolean isPublishBuildInfo(ArtifactoryClientConfiguration clientConf) {
        if (!clientConf.publisher.isPublishBuildInfo()) {
            logger.info(LOG_PREFIX + "publish build info set to false, build info will not be published...");
            return false;
        }
        return true;
    }

    /**
     * Prepare deployable artifacts by module map.
     *
     * @param modules             - The build modules
     * @param deployableArtifacts - The deployable artifacts
     * @return deployable artifacts by module map
     */
    private Map<String, Set<DeployDetails>> prepareDeployableArtifacts(List<Module> modules, Map<String, DeployDetails> deployableArtifacts) {
        Map<String, Set<DeployDetails>> deployableArtifactsByModule = new LinkedHashMap<>();
        for (Module module : modules) {

            // Create deployable artifacts set for current module
            Set<DeployDetails> moduleDeployableArtifacts = new LinkedHashSet<>();
            for (Artifact moduleArtifact : ListUtils.emptyIfNull(module.getArtifacts())) {
                String artifactId = BuildInfoExtractorUtils.getArtifactId(module.getId(), moduleArtifact.getName());
                DeployDetails deployableArtifact = deployableArtifacts.get(artifactId);
                if (deployableArtifact != null) {
                    moduleDeployableArtifacts.add(createDeployDetails(deployableArtifact, moduleArtifact));
                }
            }

            // If the deployable artifacts set is not empty, add it to the results
            if (!moduleDeployableArtifacts.isEmpty()) {
                deployableArtifactsByModule.put(module.getId(), moduleDeployableArtifacts);
            }
        }
        return deployableArtifactsByModule;
    }

    /**
     * Create DeployDetails of an artifact to deploy.
     *
     * @param deployableArtifact - DeployDetails without sha1
     * @param artifact           - Thw artifact to deploy
     * @return DeployDetails of artifact to deploy
     */
    private DeployDetails createDeployDetails(DeployDetails deployableArtifact, Artifact artifact) {
        setChecksums(deployableArtifact.getFile(), artifact, logger);
        return new DeployDetails.Builder().
                artifactPath(deployableArtifact.getArtifactPath()).
                file(deployableArtifact.getFile()).
                md5(artifact.getMd5()).
                sha1(artifact.getSha1()).
                addProperties(deployableArtifact.getProperties()).
                targetRepository(deployableArtifact.getTargetRepository()).
                packageType(DeployDetails.PackageType.MAVEN).
                build();
    }
}
