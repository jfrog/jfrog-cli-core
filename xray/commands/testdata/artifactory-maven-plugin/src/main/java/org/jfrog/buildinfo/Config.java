package org.jfrog.buildinfo;

import lombok.experimental.Delegate;
import org.jfrog.build.api.util.NullLog;
import org.jfrog.build.extractor.clientConfiguration.ArtifactoryClientConfiguration;
import org.jfrog.build.extractor.clientConfiguration.PrefixPropertyHandler;

/**
 * Represents the Artifactory Maven plugin configuration in the pom.xml file.
 * The configuration is automatically injected to this class.
 *
 * @author yahavi
 */
public class Config {

    private static final ArtifactoryClientConfiguration CLIENT_CONFIGURATION = new ArtifactoryClientConfiguration(new NullLog());

    /**
     * Represents the 'artifactory' configuration in the pom.xml.
     */
    public static class Artifactory {
        @Delegate
        ArtifactoryClientConfiguration delegate = CLIENT_CONFIGURATION;
    }

    /**
     * Represents the 'resolver' configuration in the pom.xml.
     */
    public static class Resolver {
        @Delegate(types = {
                ArtifactoryClientConfiguration.ResolverHandler.class,
                ArtifactoryClientConfiguration.RepositoryConfiguration.class,
                ArtifactoryClientConfiguration.AuthenticationConfiguration.class,
                PrefixPropertyHandler.class})
        ArtifactoryClientConfiguration.ResolverHandler delegate = CLIENT_CONFIGURATION.resolver;
    }

    /**
     * Represents the 'publisher' configuration in the pom.xml.
     */
    public static class Publisher {
        @Delegate(types = {
                ArtifactoryClientConfiguration.PublisherHandler.class,
                ArtifactoryClientConfiguration.RepositoryConfiguration.class,
                ArtifactoryClientConfiguration.AuthenticationConfiguration.class,
                PrefixPropertyHandler.class})
        ArtifactoryClientConfiguration.PublisherHandler delegate = CLIENT_CONFIGURATION.publisher;
    }

    /**
     * Represents the 'buildInfo' configuration in the pom.xml.
     */
    public static class BuildInfo {
        @Delegate(types = {ArtifactoryClientConfiguration.BuildInfoHandler.class, PrefixPropertyHandler.class})
        ArtifactoryClientConfiguration.BuildInfoHandler delegate = CLIENT_CONFIGURATION.info;
    }
}
