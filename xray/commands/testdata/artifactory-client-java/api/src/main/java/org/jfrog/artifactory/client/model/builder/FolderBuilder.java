package org.jfrog.artifactory.client.model.builder;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import org.jfrog.artifactory.client.model.Folder;
import org.jfrog.artifactory.client.model.Item;

import java.util.Date;
import java.util.List;

/**
 * @author jbaruch
 * @since 13/08/12
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public interface FolderBuilder {

    FolderBuilder uri(String uri);

    FolderBuilder repo(String repo);

    FolderBuilder path(String path);

    FolderBuilder created(Date created);

    FolderBuilder createdBy(String createdBy);

    FolderBuilder modifiedBy(String modifiedBy);

    FolderBuilder lastModified(Date lastModified);

    FolderBuilder lastUpdated(Date lastUpdated);

    FolderBuilder metadataUri(String metadataUri);

    FolderBuilder children(List<Item> children);

    Folder build();

}
