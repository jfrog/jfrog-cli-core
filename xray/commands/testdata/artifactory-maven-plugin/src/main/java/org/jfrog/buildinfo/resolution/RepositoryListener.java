package org.jfrog.buildinfo.resolution;

import org.apache.commons.lang3.StringUtils;
import org.apache.maven.artifact.Artifact;
import org.apache.maven.artifact.DefaultArtifact;
import org.codehaus.plexus.logging.Logger;
import org.eclipse.aether.AbstractRepositoryListener;
import org.eclipse.aether.RepositoryEvent;
import org.eclipse.aether.resolution.ArtifactRequest;
import org.jfrog.buildinfo.deployment.BuildInfoRecorder;

import javax.inject.Inject;
import javax.inject.Named;
import javax.inject.Singleton;

/**
 * Resolve build time dependencies. Useful if recordAllDependencies=true.
 *
 * @author yahavi
 */
@Named
@Singleton
public class RepositoryListener extends AbstractRepositoryListener {

    private BuildInfoRecorder buildInfoRecorder;

    /**
     * Empty constructor for serialization
     */
    @SuppressWarnings("unused")
    public RepositoryListener() {
    }

    /**
     * Constructor for tests
     *
     * @param logger - The logger
     */
    public RepositoryListener(Logger logger) {
        this.logger = logger;
    }

    @Inject
    private Logger logger;

    public void setBuildInfoRecorder(BuildInfoRecorder buildInfoRecorder) {
        this.buildInfoRecorder = buildInfoRecorder;
    }

    @Override
    public void artifactResolved(RepositoryEvent event) {
        if (buildInfoRecorder == null) {
            return;
        }
        String requestContext = ((ArtifactRequest) event.getTrace().getData()).getRequestContext();
        String scope = getScopeByRequestContext(requestContext);
        Artifact artifact = toMavenArtifact(event.getArtifact(), scope);
        if (artifact == null) {
            return;
        }
        if (event.getRepository() != null) {
            logger.debug("[buildinfo] Resolved artifact: " + artifact + " from: " + event.getRepository() + ". Context is: " + requestContext);
            buildInfoRecorder.artifactResolved(artifact);
            return;
        }
        logger.debug("[buildinfo] Could not resolve artifact: " + artifact);
    }

    /**
     * Converts org.eclipse.aether.artifact.Artifact objects into org.apache.maven.artifact.Artifact objects.
     */
    private Artifact toMavenArtifact(final org.eclipse.aether.artifact.Artifact art, String scope) {
        if (art == null) {
            return null;
        }
        String classifier = StringUtils.defaultString(art.getClassifier());
        DefaultArtifact artifact = new DefaultArtifact(art.getGroupId(), art.getArtifactId(), art.getVersion(), scope, art.getExtension(), classifier, null);
        artifact.setFile(art.getFile());
        return artifact;
    }

    private String getScopeByRequestContext(String requestContext) {
        return StringUtils.equals(requestContext, "plugin") ? "build" : "project";
    }
}
