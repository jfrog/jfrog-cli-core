package java

//func TestMavenTreesMultiModule(t *testing.T) {
//	// Create and change directory to test workspace
//	_, cleanUp := sca.CreateTestWorkspace(t, "maven-example")
//	defer cleanUp()
//
//	expectedUniqueDeps := []string{
//		GavPackageTypeIdentifier + "javax.mail:mail:1.4",
//		GavPackageTypeIdentifier + "org.testng:testng:5.9",
//		GavPackageTypeIdentifier + "javax.servlet:servlet-api:2.5",
//		GavPackageTypeIdentifier + "org.jfrog.test:multi:3.7-SNAPSHOT",
//		GavPackageTypeIdentifier + "org.jfrog.test:multi3:3.7-SNAPSHOT",
//		GavPackageTypeIdentifier + "org.jfrog.test:multi2:3.7-SNAPSHOT",
//		GavPackageTypeIdentifier + "junit:junit:3.8.1",
//		GavPackageTypeIdentifier + "org.jfrog.test:multi1:3.7-SNAPSHOT",
//		GavPackageTypeIdentifier + "commons-io:commons-io:1.4",
//		GavPackageTypeIdentifier + "org.apache.commons:commons-email:1.1",
//		GavPackageTypeIdentifier + "javax.activation:activation:1.1",
//		GavPackageTypeIdentifier + "hsqldb:hsqldb:1.8.0.10",
//	}
//	// Run getModulesDependencyTrees
//	modulesDependencyTrees, uniqueDeps, err := buildMvnDependencyTree(&DepTreeParams{IgnoreConfigFile: true})
//	if assert.NoError(t, err) && assert.NotEmpty(t, modulesDependencyTrees) {
//		assert.ElementsMatch(t, uniqueDeps, expectedUniqueDeps, "First is actual, Second is Expected")
//		// Check root module
//		multi := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi:3.7-SNAPSHOT")
//		if assert.NotNil(t, multi) {
//			assert.Empty(t, multi.Nodes)
//			// Check multi1 with a transitive dependency
//			multi1 := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi1:3.7-SNAPSHOT")
//			assert.Len(t, multi1.Nodes, 4)
//			commonsEmail := sca.GetAndAssertNode(t, multi1.Nodes, "org.apache.commons:commons-email:1.1")
//			assert.Len(t, commonsEmail.Nodes, 2)
//
//			// Check multi2 and multi3
//			multi2 := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi2:3.7-SNAPSHOT")
//			assert.Len(t, multi2.Nodes, 1)
//			multi3 := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi3:3.7-SNAPSHOT")
//			assert.Len(t, multi3.Nodes, 4)
//		}
//	}
//}
//
//func TestMavenWrapperTrees(t *testing.T) {
//	// Create and change directory to test workspace
//	_, cleanUp := sca.CreateTestWorkspace(t, "maven-example-with-wrapper")
//	err := os.Chmod("mvnw", 0700)
//	defer cleanUp()
//	assert.NoError(t, err)
//	expectedUniqueDeps := []string{
//		GavPackageTypeIdentifier + "org.jfrog.test:multi1:3.7-SNAPSHOT",
//		GavPackageTypeIdentifier + "org.codehaus.plexus:plexus-utils:1.5.1",
//		GavPackageTypeIdentifier + "org.springframework:spring-beans:2.5.6",
//		GavPackageTypeIdentifier + "commons-logging:commons-logging:1.1.1",
//		GavPackageTypeIdentifier + "org.jfrog.test:multi3:3.7-SNAPSHOT",
//		GavPackageTypeIdentifier + "org.apache.commons:commons-email:1.1",
//		GavPackageTypeIdentifier + "org.springframework:spring-aop:2.5.6",
//		GavPackageTypeIdentifier + "org.springframework:spring-core:2.5.6",
//		GavPackageTypeIdentifier + "org.jfrog.test:multi:3.7-SNAPSHOT",
//		GavPackageTypeIdentifier + "org.jfrog.test:multi2:3.7-SNAPSHOT",
//		GavPackageTypeIdentifier + "org.testng:testng:5.9",
//		GavPackageTypeIdentifier + "hsqldb:hsqldb:1.8.0.10",
//		GavPackageTypeIdentifier + "junit:junit:3.8.1",
//		GavPackageTypeIdentifier + "javax.activation:activation:1.1",
//		GavPackageTypeIdentifier + "javax.mail:mail:1.4",
//		GavPackageTypeIdentifier + "aopalliance:aopalliance:1.0",
//		GavPackageTypeIdentifier + "commons-io:commons-io:1.4",
//		GavPackageTypeIdentifier + "javax.servlet.jsp:jsp-api:2.1",
//		GavPackageTypeIdentifier + "javax.servlet:servlet-api:2.5",
//	}
//
//	modulesDependencyTrees, uniqueDeps, err := buildMvnDependencyTree(&DepTreeParams{IgnoreConfigFile: true, UseWrapper: true})
//	if assert.NoError(t, err) && assert.NotEmpty(t, modulesDependencyTrees) {
//		assert.ElementsMatch(t, uniqueDeps, expectedUniqueDeps, "First is actual, Second is Expected")
//		// Check root module
//		multi := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi:3.7-SNAPSHOT")
//		if assert.NotNil(t, multi) {
//			assert.Empty(t, multi.Nodes)
//			// Check multi1 with a transitive dependency
//			multi1 := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi1:3.7-SNAPSHOT")
//			assert.Len(t, multi1.Nodes, 7)
//			commonsEmail := sca.GetAndAssertNode(t, multi1.Nodes, "org.apache.commons:commons-email:1.1")
//			assert.Len(t, commonsEmail.Nodes, 2)
//			// Check multi2 and multi3
//			multi2 := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi2:3.7-SNAPSHOT")
//			assert.Len(t, multi2.Nodes, 1)
//			multi3 := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi3:3.7-SNAPSHOT")
//			assert.Len(t, multi3.Nodes, 4)
//		}
//	}
//}
