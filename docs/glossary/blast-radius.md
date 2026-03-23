# Blast Radius

The scope of potential damage if an AI agent behaves unexpectedly -- accessing unauthorized resources, leaking secrets, modifying files outside its intended scope, or making unintended network requests.

Without containment, an agent's blast radius equals the user's full system access (filesystem, network, processes, secrets). Airlock reduces blast radius to the contents of a single Docker container with no access to host secrets in plaintext.
