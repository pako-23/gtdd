package testsuite

import (
	"fmt"
	"github.com/pako-23/gtdd/internal/docker"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
)

const listTestsScript = `
find target/ -name TEST*.xml -exec grep testcase {} \\; | awk -F'\"' '{
    for (i = 1; i <= NF; i++) {
      if (\$i ~ /classname=/) {
         classname=\$(i+1)
      } else if (\$i ~ /name=/) {
         name=\$(i+1)
      }
    }
    if (classname && name) { print classname \"#\" name }
  }' | uniq`

const junitRunner = `import org.junit.runner.JUnitCore;
import org.junit.runner.Request;
import org.junit.runner.Result;
import java.io.PrintWriter;
import java.io.FileWriter;
import java.io.IOException;

public class CustomRunner {
    public static void main(final String[] args) throws ClassNotFoundException {
        JUnitCore core = new JUnitCore();
        final boolean[] results = new boolean[args.length];

	for (int i = 0; i < args.length; ++i) {
	    String[] classAndMethod = args[i].split("#");
            Request request = Request.method(Class.forName(classAndMethod[0]),
               classAndMethod[1]);

	    Result result = core.run(request);
	    results[i] = result.wasSuccessful();
        }

	try {
	    PrintWriter out = new PrintWriter(new FileWriter("summary.txt"));

	    for (int i = 0; i < results.length; ++i)
		out.println(String.format("%s %d", args[i], results[i] ? 1 : 0));

	    out.close();
        } catch (IOException e) {
            System.exit(1);
        }
        System.exit(0);
    }
}`

const dockerFile = `FROM maven:3.6.1-jdk-8

COPY %s/ /app
WORKDIR /app

RUN curl -O https://repo1.maven.org/maven2/junit/junit/4.12/junit-4.12.jar
RUN mvn clean test
RUN mvn dependency:build-classpath -DincludeScope=test -Dmdep.outputFile=cp.txt
RUN echo "#\!/bin/sh\n%s"  > ./list_tests.sh
RUN chmod +x list_tests.sh
RUN echo '%s' > CustomRunner.java
RUN javac -cp "/app/junit-4.12.jar:$(cat cp.txt):" CustomRunner.java
RUN echo "#\!/bin/sh\n\njava -cp \"/app/junit-4.12.jar:$(cat cp.txt):/app/target/test-classes/:/app/target/classes/:\" CustomRunner \"\$@\" >/dev/null\ncat summary.txt" > run_tests.sh
RUN chmod +x run_tests.sh
`

type JunitTestSuite struct {
	Image string
}

func (j *JunitTestSuite) Build(path string) error {
	file, err := os.Create("Dockerfile")
	if err != nil {
		return err
	}
	defer os.Remove("Dockerfile")

	fmt.Fprintf(file, dockerFile,
		path,
		strings.ReplaceAll(listTestsScript, "\n", "\\n"),
		strings.ReplaceAll(junitRunner, "\n", "\\n"))
	file.Close()

	client, err := docker.NewClient()
	if err != err {
		return err
	}
	defer client.Close()

	return client.BuildImage(j.Image, ".", "Dockerfile")
}

func (j *JunitTestSuite) ListTests() (tests []string, err error) {
	client, err := docker.NewClient()
	if err != err {
		return nil, err
	}
	defer client.Close()

	app := docker.App{
		"testsuite": {Command: []string{"./list_tests.sh"}, Image: j.Image},
	}
	instance, err := client.Run(app, docker.RunOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start Java test suite container: %w", err)
	}
	defer func() {
		deleteErr := client.Delete(instance)
		if err == nil {
			err = deleteErr

		}
	}()

	logs, err := client.GetContainerLogs(instance["testsuite"])
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.Trim(logs, "\n"), "\n"), nil
}

func (j *JunitTestSuite) Run(config *RunConfig) (results []bool, err error) {
	client, err := docker.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create cleint for JUnit testsuite: %w", err)
	}

	suite := docker.App{
		config.Name: {
			Command:     append([]string{"./run_tests.sh"}, config.Tests...),
			Image:       j.Image,
			Environment: config.Env,
		},
	}

	instance, err := client.Run(suite, *config.StartConfig)
	if err != nil {
		return nil, fmt.Errorf("error in starting java test suite container: %w", err)
	}
	defer func() {
		deleteErr := client.Delete(instance)
		if err == nil {
			err = deleteErr

		}
	}()
	log.Debugf("successfully started java test suite container %s", instance[config.Name])

	logs, err := client.GetContainerLogs(instance[config.Name])
	if err != nil {
		return nil, err
	}

	log.Debugf("successfully obtained logs from java test suite container %s", instance["testsuite"])
	log.Debugf("container logs: %s", logs)

	result := make([]bool, len(config.Tests))
	lines := strings.Split(strings.Trim(logs, "\n"), "\n")
	for i, line := range lines {
		result[i] = line[len(line)-1] == '1'
	}

	return result, nil
}
