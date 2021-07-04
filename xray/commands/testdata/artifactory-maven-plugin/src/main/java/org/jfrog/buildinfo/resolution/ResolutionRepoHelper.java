package org.jfrog.buildinfo.resolution;

import com.google.common.collect.Lists;
import org.apache.commons.lang3.StringUtils;
import org.apache.maven.artifact.repository.ArtifactRepository;
import org.apache.maven.artifact.repository.ArtifactRepositoryPolicy;
import org.apache.maven.artifact.repository.Authentication;
import org.apache.maven.artifact.repository.MavenArtifactRepository;
import org.apache.maven.artifact.repository.layout.DefaultRepositoryLayout;
import org.apache.maven.execution.MavenSession;
import org.apache.maven.plugin.logging.Log;
import org.jfrog.build.extractor.BuildInfoExtractorUtils;
import org.jfrog.build.extractor.clientConfiguration.ArtifactoryClientConfiguration;
import org.jfrog.buildinfo.utils.ArtifactoryMavenLogger;

import java.util.List;
import java.util.Properties;

/**
 * Create Artifactory resolution repositories list.
 *
 * @author yahavi
 */
public class ResolutionRepoHelper {

    private final ArtifactoryClientConfiguration clientConfiguration;
    private final Log logger;

    public ResolutionRepoHelper(Log logger, MavenSession session, ArtifactoryClientConfiguration clientConfiguration) {
        this.logger = logger;
        this.clientConfiguration = clientConfiguration;
        Properties allMavenProps = new Properties() {{
            putAll(session.getSystemProperties());
            putAll(session.getUserProperties());
        }};
        Properties allProps = BuildInfoExtractorUtils.mergePropertiesWithSystemAndPropertyFile(allMavenProps, new ArtifactoryMavenLogger(logger));
        this.clientConfiguration.fillFromProperties(allProps);
    }

    /**
     * Get Artifactory resolution repositories.
     *
     * @return Artifactory resolution repositories
     */
    public List<ArtifactRepository> getResolutionRepositories() {
        List<ArtifactRepository> resolutionRepositories = Lists.newArrayList();
        String snapshotRepoUrl = getRepoSnapshotUrl();
        String releaseRepoUrl = getRepoReleaseUrl();
        String username = getRepoUsername();
        Authentication authentication = StringUtils.isNotBlank(username) ? new Authentication(username, getRepoPassword()) : null;

        // Add snapshots repository
        if (StringUtils.isNotBlank(snapshotRepoUrl)) {
            resolutionRepositories.add(createSnapshotsRepository(snapshotRepoUrl, authentication));
        }

        // Add releases repository
        if (StringUtils.isNotBlank(releaseRepoUrl)) {
            resolutionRepositories.add(createReleasesRepository(releaseRepoUrl, authentication, resolutionRepositories.isEmpty()));
        }

        return resolutionRepositories;
    }

    /**
     * Create artifactory snapshots resolution repository.
     *
     * @param repoUrl        - The repository URL
     * @param authentication - The repository credentials
     * @return artifactory snapshots resolution repository
     */
    private ArtifactRepository createSnapshotsRepository(String repoUrl, Authentication authentication) {
        logger.debug("[buildinfo] Enforcing snapshot repository for resolution: " + repoUrl);
        ArtifactRepositoryPolicy releasePolicy = new ArtifactRepositoryPolicy(false, ArtifactRepositoryPolicy.UPDATE_POLICY_DAILY, ArtifactRepositoryPolicy.CHECKSUM_POLICY_WARN);
        ArtifactRepositoryPolicy snapshotPolicy = new ArtifactRepositoryPolicy(true, ArtifactRepositoryPolicy.UPDATE_POLICY_DAILY, ArtifactRepositoryPolicy.CHECKSUM_POLICY_WARN);
        ArtifactRepository snapshotRepository = new MavenArtifactRepository("artifactory-snapshot", repoUrl, new DefaultRepositoryLayout(), snapshotPolicy, releasePolicy);
        if (authentication != null) {
            logger.debug("[buildinfo] Enforcing repository authentication: " + authentication + " for snapshot resolution repository");
            snapshotRepository.setAuthentication(authentication);
        }
        return snapshotRepository;
    }

    /**
     * Create artifactory releases resolution repository.
     *
     * @param repoUrl               - The repository URL
     * @param authentication        - The repository credentials
     * @param snapshotPolicyEnabled - True to resolve both snapshots and releases dependencies
     * @return artifactory snapshots resolution repository
     */
    private ArtifactRepository createReleasesRepository(String repoUrl, Authentication authentication, boolean snapshotPolicyEnabled) {
        logger.debug("[buildinfo] Enforcing release repository for resolution: " + repoUrl);
        String repositoryId = snapshotPolicyEnabled ? "artifactory-release-snapshot" : "artifactory-release";

        ArtifactRepositoryPolicy releasePolicy = new ArtifactRepositoryPolicy(true, ArtifactRepositoryPolicy.UPDATE_POLICY_DAILY, ArtifactRepositoryPolicy.CHECKSUM_POLICY_WARN);
        ArtifactRepositoryPolicy snapshotPolicy = new ArtifactRepositoryPolicy(snapshotPolicyEnabled, ArtifactRepositoryPolicy.UPDATE_POLICY_DAILY, ArtifactRepositoryPolicy.CHECKSUM_POLICY_WARN);
        ArtifactRepository releasePluginRepository = new MavenArtifactRepository(repositoryId, repoUrl, new DefaultRepositoryLayout(), snapshotPolicy, releasePolicy);
        if (authentication != null) {
            logger.debug("[buildinfo] Enforcing repository authentication: " + authentication + " for release resolution repository");
            releasePluginRepository.setAuthentication(authentication);
        }
        return releasePluginRepository;
    }

    private String getRepoReleaseUrl() {
        return clientConfiguration.resolver.getUrl(clientConfiguration.resolver.getRepoKey());
    }

    private String getRepoSnapshotUrl() {
        return clientConfiguration.resolver.getUrl(clientConfiguration.resolver.getDownloadSnapshotRepoKey());
    }

    private String getRepoUsername() {
        return clientConfiguration.resolver.getUsername();
    }

    private String getRepoPassword() {
        return clientConfiguration.resolver.getPassword();
    }

}
