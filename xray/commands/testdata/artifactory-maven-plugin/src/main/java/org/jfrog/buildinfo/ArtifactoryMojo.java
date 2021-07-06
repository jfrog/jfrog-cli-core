package org.jfrog.buildinfo;

import org.apache.commons.collections4.CollectionUtils;
import org.apache.commons.lang3.ArrayUtils;
import org.apache.commons.lang3.StringUtils;
import org.apache.maven.artifact.repository.ArtifactRepository;
import org.apache.maven.execution.MavenSession;
import org.apache.maven.plugin.AbstractMojo;
import org.apache.maven.plugins.annotations.Component;
import org.apache.maven.plugins.annotations.LifecyclePhase;
import org.apache.maven.plugins.annotations.Mojo;
import org.apache.maven.plugins.annotations.Parameter;
import org.apache.maven.project.MavenProject;
import org.jfrog.build.api.BuildInfoFields;
import org.jfrog.build.extractor.clientConfiguration.ArtifactoryClientConfiguration;
import org.jfrog.build.extractor.clientConfiguration.ClientProperties;
import org.jfrog.buildinfo.deployment.BuildInfoRecorder;
import org.jfrog.buildinfo.resolution.RepositoryListener;
import org.jfrog.buildinfo.resolution.ResolutionRepoHelper;
import org.jfrog.buildinfo.utils.Utils;

import java.text.SimpleDateFormat;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Properties;

import static org.jfrog.buildinfo.utils.Utils.getMavenVersion;

/**
 * The plugin's entry point -
 * Replace the resolution repositories.
 * Replace the default deployment with the BuildInfoRecorder.
 */
@Mojo(name = "publish", defaultPhase = LifecyclePhase.VALIDATE, threadSafe = true)
public class ArtifactoryMojo extends AbstractMojo {
    private static final SimpleDateFormat DATE_FORMAT = new SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSSZ");
    private static final String[] DEPLOY_GOALS = {"deploy", "maven-deploy-plugin"};

    @Parameter(required = true, defaultValue = "${project}")
    MavenProject project;

    @Parameter(required = true, defaultValue = "${session}")
    MavenSession session;

    @Component(role = RepositoryListener.class)
    RepositoryListener repositoryListener;

    @Parameter
    Config.Artifactory artifactory = new Config.Artifactory();

    @Parameter
    Map<String, String> deployProperties = new HashMap<>();

    @Parameter
    Config.BuildInfo buildInfo = new Config.BuildInfo();

    @Parameter
    Config.Publisher publisher = new Config.Publisher();

    @Parameter
    Config.Resolver resolver = new Config.Resolver();

    @Override
    public void execute() {
        if (session.getRequest().getData().putIfAbsent("configured", Boolean.TRUE) == null) {
            replaceVariables();
            enforceResolution();
            enforceDeployment();
        }
    }

    /**
     * Replace variables in pom.xml surrounded by {{}} with environment and system variables.
     */
    private void replaceVariables() {
        artifactory.delegate.getAllProperties().replaceAll((key, value) -> Utils.parseInput(value));
        deployProperties.replaceAll((key, value) -> Utils.parseInput(value));
    }

    /**
     * Enforce resolution from Artifactory repositories if there is a 'resolver' configuration.
     */
    private void enforceResolution() {
        ResolutionRepoHelper helper = new ResolutionRepoHelper(getLog(), session, artifactory.delegate);
        List<ArtifactRepository> resolutionRepositories = helper.getResolutionRepositories();
        if (CollectionUtils.isNotEmpty(resolutionRepositories)) {
            // User configured the 'resolver'
            for (MavenProject mavenProject : session.getProjects()) {
                mavenProject.setPluginArtifactRepositories(resolutionRepositories);
                mavenProject.setRemoteArtifactRepositories(resolutionRepositories);
            }
        }
    }

    /**
     * Enforce deployment to Artifactory repositories if there is a 'publisher' configuration.
     */
    private void enforceDeployment() {
        if (!deployGoalExist()) {
            return;
        }
        skipDefaultDeploy();
        completeConfig();
        addDeployProperties();
        BuildInfoRecorder executionListener = new BuildInfoRecorder(session, getLog(), artifactory.delegate);
        repositoryListener.setBuildInfoRecorder(executionListener);
        session.getRequest().setExecutionListener(executionListener);
    }

    /**
     * Return true if 'deploy' or 'maven-deploy-plugin' goals exist.
     *
     * @return true if 'deploy' or 'maven-deploy-plugin' goals exist
     */
    private boolean deployGoalExist() {
        return session.getGoals().stream().anyMatch(goal -> ArrayUtils.contains(DEPLOY_GOALS, goal));
    }

    /**
     * Skip the default maven deploy behaviour.
     */
    private void skipDefaultDeploy() {
        session.getUserProperties().put("maven.deploy.skip", Boolean.TRUE.toString());
    }

    /**
     * Complete missing configuration.
     */
    private void completeConfig() {
        ArtifactoryClientConfiguration.BuildInfoHandler buildInfo = this.buildInfo.delegate;
        buildInfo.setBuildTimestamp(Long.toString(session.getStartTime().getTime()));
        buildInfo.setBuildStarted(DATE_FORMAT.format(session.getStartTime()));
        if (StringUtils.isBlank(buildInfo.getBuildName())) {
            buildInfo.setBuildName(project.getArtifactId());
        }
        if (StringUtils.isBlank(buildInfo.getBuildNumber())) {
            buildInfo.setBuildNumber(buildInfo.getBuildTimestamp());
        }
        buildInfo.setBuildAgentName("Maven");
        buildInfo.setBuildAgentVersion(getMavenVersion(getClass()));
        if (buildInfo.getBuildRetentionDays() != null) {
            buildInfo.setBuildRetentionMinimumDate(buildInfo.getBuildRetentionDays().toString());
        }
    }

    /**
     * Add buildName, buildNumber, the timestamp and the configured deployProperties as a deployment properties.
     * The deployer adds these properties to each of of the deployed artifacts.
     */
    private void addDeployProperties() {
        ArtifactoryClientConfiguration.BuildInfoHandler buildInfo = this.buildInfo.delegate;
        Properties deployProperties = new Properties() {{
            addDeployProperty(this, BuildInfoFields.BUILD_TIMESTAMP, buildInfo.getBuildTimestamp());
            addDeployProperty(this, BuildInfoFields.BUILD_NAME, buildInfo.getBuildName());
            addDeployProperty(this, BuildInfoFields.BUILD_NUMBER, buildInfo.getBuildNumber());
        }};
        this.deployProperties.forEach((key, value) -> addDeployProperty(deployProperties, key, value));
        artifactory.delegate.fillFromProperties(deployProperties);
    }

    /**
     * Add a single deploy property.
     *
     * @param deployProperties - The deploy properties collection
     * @param key              - The key of the property
     * @param value            - The value of the property
     */
    private void addDeployProperty(Properties deployProperties, String key, String value) {
        if (StringUtils.isNotBlank(value)) {
            deployProperties.put(ClientProperties.PROP_DEPLOY_PARAM_PROP_PREFIX + key, value);
        }
    }
}
