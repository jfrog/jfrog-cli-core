package org.jfrog.artifactory.client.model.impl;

import org.jfrog.artifactory.client.model.Item;

import java.io.File;
import java.util.Date;

/**
 * @author jbaruch
 * @since 29/07/12
 */

public class ItemImpl implements Item {
    private String uri;
    private String repo;
    private String path;
    private boolean folder;
    private String metadataUri;
    private Date lastModified;
    private String modifiedBy;
    private Date lastUpdated;


    protected ItemImpl(boolean folder, String uri, String metadataUri, Date lastModified, String modifiedBy, Date lastUpdated) {
        this.folder = folder;
        this.uri = uri;
        this.metadataUri = metadataUri;
        this.lastModified = lastModified;
        this.modifiedBy = modifiedBy;
        this.lastUpdated = lastUpdated;
    }

    protected ItemImpl() {
    }

    protected ItemImpl(boolean folder, String uri){
        this.folder = folder;
        this.uri = uri;
    }

    public String getPath() {
        return path;
    }

    private void setPath(String path) {
        this.path = path;
    }

    public String getRepo() {
        return repo;
    }

    private void setRepo(String repo) {
        this.repo = repo;
    }

    public boolean isFolder() {
        return folder;
    }

    private void setFolder(boolean folder) {
        this.folder = folder;
    }

    public String getName() {
        return new File(uri).getName();
    }

    public String getUri() {
        return uri;
    }

    private void setUri(String uri) {
        this.uri = uri;
    }

    public String getMetadataUri() {
        return metadataUri;
    }

    private void setMetadataUri(String metadataUri) {
        this.metadataUri = metadataUri;
    }

    public Date getLastModified() {
        return lastModified;
    }

    private void setLastModified(Date lastModified) {
        this.lastModified = lastModified;
    }

    public String getModifiedBy() {
        return modifiedBy;
    }

    private void setModifiedBy(String modifiedBy) {
        this.modifiedBy = modifiedBy;
    }

    public Date getLastUpdated() {
        return lastUpdated;
    }

    private void setLastUpdated(Date lastUpdated) {
        this.lastUpdated = lastUpdated;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) {
            return true;
        }
        if (!(o instanceof ItemImpl)) {
            return false;
        }

        ItemImpl item = (ItemImpl) o;

        if (!uri.equals(item.uri)) {
            return false;
        }

        return true;
    }

    @Override
    public int hashCode() {
        return uri.hashCode();
    }
}