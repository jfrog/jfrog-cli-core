package org.jfrog.artifactory.client;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import org.jfrog.artifactory.client.model.Folder;
import org.jfrog.artifactory.client.model.Item;
import org.jfrog.artifactory.client.model.ItemPermission;

import java.io.IOException;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * @author jbaruch
 * @since 12/08/12
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public interface ItemHandle {

    <T extends Item> T info();

    boolean isFolder();

    Map<String, List<String>> getProperties(String... properties);

    List<String> getPropertyValues(String propertyName);

    PropertiesHandler properties();

    Map<String, ?> deleteProperty(String property);

    Map<String, ?> deleteProperties(String... properties);

    Set<ItemPermission> effectivePermissions();

    ItemHandle move(String toRepo, String toPath);

    ItemHandle copy(String toRepo, String toPath);

    Folder create() throws IOException;
}