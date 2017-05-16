# argo-lyte

## What does this do?
This program pulls down the argonauts file from an S3 bucket, un-tars it, and proceeds to create groups and users(associating groups and ssh keys as well)

### Read thru groups directory and create new groups.
1. Reads in json group file.
2. Execs out and creates group via groupadd.
3. As you create the group, loop thru the users in the json file and create a map containing the user as the key and the groups as the value
4. Write out each group to leveldb so the next time the code is run, it can determine what has changed

### Read thru users directory and create new users.
1. Reads in json user file.
2. Exec out and create the user via useradd with the correct shell and groups
3. Write out each user and their groups to leveldb so the next time the code is run, it can determine what has changed
4. Write out each user and their keys to leveldb so the next time the code is run, it can determine what has changed
5. Create the .ssh directory in the users directory with the correct permissions
6. Create the authorized_key file in the ssh directory

### Read thru user files and compare the users to the users in leveldb to see if a new user was added or removed and check and see if users groups have changed

### Read thru group files and compare the groups to the groups in leveldb to see if a new group was added or removed

### Things still to resolve
1. Dealing with users changing keys

### Supported Operating Systems
1. Ubuntu 14.04
2. Ubuntu 16.04