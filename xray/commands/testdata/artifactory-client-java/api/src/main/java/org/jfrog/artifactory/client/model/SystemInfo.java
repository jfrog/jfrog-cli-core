package org.jfrog.artifactory.client.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;

/**
 * Created by Ivan Verhun on 18/12/2015.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public interface SystemInfo {

    long getCommittedVirtualMemorySize();

    long getTotalSwapSpaceSize();

    long getFreeSwapSpaceSize();

    long getProcessCpuTime();

    long getTotalPhysicalMemorySize();

    long getOpenFileDescriptorCount();

    long getMaxFileDescriptorCount();

    double getProcessCpuLoad();

    double getSystemCpuLoad();

    long getFreePhysicalMemorySize();

    long getNumberOfCores();

    long getHeapMemoryUsage();

    long getNoneHeapMemoryUsage();

    long getThreadCount();

    long getNoneHeapMemoryMax();

    long getHeapMemoryMax();

    long getJvmUpTime();
}
