# How to contribute to Nalej

## **Did you find a bug?**

* **Do not open up a GitHub issue if the bug is a security vulnerability
  in Nalej**, please send an email to our [support](mailto:support@nalej.com) team.

* **Ensure the bug was not already reported** by searching on GitHub under the issues section in the corresponding repository. For example, https://github.com/nalej/conductor/issues

* If you're unable to find an open issue addressing the problem, [open a new one](https://github.com/nalej/conductor/issues/new). Be sure to include a **title and clear description**, as much relevant information as possible, and a **code sample** or an **executable test case** demonstrating the expected behavior that is not occurring.

* If possible, use the bug template below to create the issue. Fill the template and give as many details as you can to help others clarifying the issue.

```
## Expected Behavior

## Actual Behavior

## Steps to Reproduce the Problem
1.
2.
3.

## Specifications
- Version:
- K8s version:
- Subsystem:
- Browser:
```
## **Did you write a patch that fixes a bug?**

* Fork the repository or repositories you want to work with. Add your code.

* Once you think your code is ready fill a Pull Request to the original Nalej repo. Open a Pull Request and fill the corresponding Nalej PR form (already included into every repo).

```
#### What does this PR do?

#### Where should the reviewer start?

#### What is missing?

#### How should this be manually tested?

#### Any background context you want to provide?

#### What are the relevant tickets? (if proceed)

- [NP-XXX](https://nalej.atlassian.net/browse/NP-XXX)

#### Screenshots (if appropriate)

#### Questions
```
* Ensure the PR description clearly describes the problem and solution. Include the relevant issue number if applicable. Do not forget to mention the steps to be carried out in order to test the expected behavior of the patch.

* If your changes are accepted your contribution will become part of the Nalej repo.

### **Style and code formatting**

* Nalej follows the [Golang code style guide](https://golang.org/doc/effective_go.html). Use gofmt on your code to automatically fix the majority of mechanical style issues.

### **Third-party libraries**

* Third-party libraries and components must be compatible with the Nalej project license.

### **Documentation**

* Undocumented code will be automatically rejected. For more details about how to document your code check [this](https://golang.org/doc/effective_go.html#commentary).

### **Repo ownership**

* In Nalej every repo has one or several owners. No code will be merged into the master branch until at least on of the repo owners has already accepted the contribution. The owner is responsible of the correctness and the reliability of the repo following the Nalej quality standards. Please, respect owners decisions and always try to establish a fruitful and respectful discussion.

## **Do you intend to add a new feature or change an existing one?**

* Suggest your changes in the corresponding repo under the issues section. No additional changes will be considered until positive feedback has been collected from the repo owners.

* Once your proposal is discussed, positive feedback is collected and you have green light you can start follow the pull request process as described above.


## **Do you have questions about the source code?**

* Do not hesitate to contact other developers for further information and fruitful discussions.

* Address your thoughts at specific repositories to facilitate the discussion.

* Be always open and collaborative and respect other developers work.

## **Code of conduct**

* Nalej has a strict code of conduct to enforce a fluent collaboration among developers.

* Check our [code of conduct](code-of-conduct.md) for more details.


The Nalej Team
