package org.jfrog.buildinfo.types;

import org.apache.maven.artifact.Artifact;

import javax.annotation.Nonnull;
import java.util.Collection;
import java.util.HashSet;
import java.util.Set;

/**
 * Represents a thread local set of current module artifacts or dependencies.
 *
 * @author yahavi
 */
public class ModuleArtifacts extends ThreadLocal<Set<Artifact>> {

    /**
     * Get the set of artifacts.
     *
     * @return the set of current artifacts
     */
    @Nonnull
    public Set<Artifact> getOrCreate() {
        Set<Artifact> artifacts = super.get();
        if (artifacts == null) {
            artifacts = new HashSet<>();
            set(artifacts);
        }
        return artifacts;
    }

    /**
     * Add all artifacts to the set.
     *
     * @param artifactsToAdd - The artifacts to add
     */
    public void addAll(Collection<Artifact> artifactsToAdd) {
        Set<Artifact> artifacts = getOrCreate();
        artifacts.addAll(artifactsToAdd);
    }

    /**
     * Add an artifact to the set.
     *
     * @param artifactToAdd - The artifact to add
     */
    public void add(Artifact artifactToAdd) {
        Set<Artifact> artifacts = getOrCreate();
        artifacts.add(artifactToAdd);
    }
}
