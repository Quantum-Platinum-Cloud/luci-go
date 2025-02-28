-- Copyright 2022 The LUCI Authors.
-- 
-- Licensed under the Apache License, Version 2.0 (the "License");
-- you may not use this file except in compliance with the License.
-- You may obtain a copy of the License at
-- 
--      http://www.apache.org/licenses/LICENSE-2.0
-- 
-- Unless required by applicable law or agreed to in writing, software
-- distributed under the License is distributed on an "AS IS" BASIS,
-- WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
-- See the License for the specific language governing permissions and
-- limitations under the License.

--------------------------------------------------------------------------------
-- This script initializes a LUCI Analysis Spanner database.

-- Stores a test variant.
-- The test variant should be:
-- * currently flaky
-- * suspected of flakiness that needs to be verified
-- * flaky before but has been fixed, broken, disabled or removed
CREATE TABLE AnalyzedTestVariants (
  -- Security realm this test variant belongs to.
  Realm STRING(64) NOT NULL,

  -- Builder that the test variant runs on.
  -- It must have the same value as the builder variant.
  Builder STRING(MAX),

  -- Unique identifier of the test,
  -- see also luci.resultdb.v1.TestResult.test_id.
  TestId STRING(MAX) NOT NULL,

  -- key:value pairs to specify the way of running the test.
  -- See also luci.resultdb.v1.TestResult.variant.
  Variant ARRAY<STRING(MAX)>,

  -- A hex-encoded sha256 of concatenated "<key>:<value>\n" variant pairs.
  -- Combination of Realm, TestId and VariantHash can identify a test variant.
  VariantHash STRING(64) NOT NULL,

  -- Timestamp when the row of a test variant was created.
  CreateTime TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),

  -- Status of the analyzed test variant, see analyzedtestvariant.Status.
  Status INT64 NOT NULL,
  -- Timestamp when the status field was last updated.
  StatusUpdateTime TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
  -- Timestamp the next UpdateTestVariant task is enqueued.
  -- This timestamp is used as a token to validate an UpdateTestVariant is
  -- expected. A task with unmatched token will be silently ignored.
  NextUpdateTaskEnqueueTime TIMESTAMP,
  -- Previous statuses of the analyzed test variant.
  -- If the test variant is a newly detected one, or its status has not changed
  -- at all, this field is empty.
  -- With PreviousStatusUpdateTimes, they are used when exporting test variants
  -- to BigQuery, to determine the time ranges of the rows that happened when
  -- the test variant's status changed.
  PreviousStatuses ARRAY<INT64>,
  -- Previous status update times.
  -- Must have the same number of elements as PreviousStatuses.
  PreviousStatusUpdateTimes ARRAY<TIMESTAMP>,

  -- Compressed metadata for the test case.
  -- For example, the original test name, test location, etc.
  -- See TestResult.test_metadata for details.
  -- Test location is helpful for dashboards to get aggregated data by directories.
  TestMetadata BYTES(MAX),

  -- key:value pairs for the metadata of the test variant.
  -- For example the monorail component and team email.
  Tags ARRAY<STRING(MAX)>,

  -- Flake statistics, including flake rate, failure rate and counts.
  -- See FlakeStatistics proto.
  FlakeStatistics BYTES(MAX),
  -- Timestamp when the most recent flake statistics were computed.
  FlakeStatisticUpdateTime TIMESTAMP,
) PRIMARY KEY (Realm, TestId, VariantHash);

-- Used by finding test variants with FLAKY status on a builder in
-- CollectFlakeResults task.
CREATE NULL_FILTERED INDEX AnalyzedTestVariantsByBuilderAndStatus
ON AnalyzedTestVariants (Realm, Builder, Status);

-- Stores results of a test variant in one invocation.
CREATE TABLE Verdicts (
  -- Primary Key of the parent AnalyzedTestVariants.
  -- Security realm this test variant belongs to.
  Realm STRING(64) NOT NULL,
  -- Unique identifier of the test,
  -- see also luci.resultdb.v1.TestResult.test_id.
  TestId STRING(MAX) NOT NULL,
  -- A hex-encoded sha256 of concatenated "<key>:<value>\n" variant pairs.
  -- Combination of Realm, TestId and VariantHash can identify a test variant.
  VariantHash STRING(64) NOT NULL,

  -- Id of the build invocation the results belong to.
  InvocationId STRING(MAX) NOT NULL,

  -- Flag indicates if the verdict belongs to a try build.
  IsPreSubmit BOOL,

  -- Flag indicates if the try build the verdict belongs to contributes to
  -- a CL's submission.
  -- Verdicts with HasContributedToClSubmission as False will be filtered out
  -- for deciding the test variant's status because they could be noises.
  -- This field is only meaningful for PreSubmit verdicts.
  HasContributedToClSubmission BOOL,

  -- If the unexpected results in the verdict are exonerated.
  Exonerated BOOL,

  -- Status of the results for the parent test variant in this verdict,
  -- See VerdictStatus.
  Status INT64 NOT NULL,

  -- Result counts in the verdict.
  -- Note that SKIP results are ignored in either of the counts.
  UnexpectedResultCount INT64,
  TotalResultCount INT64,

  --Creation time of the invocation containing this verdict.
  InvocationCreationTime TIMESTAMP NOT NULL,

  -- Ingestion time of the verdict.
  IngestionTime TIMESTAMP NOT NULL,

  -- List of colon-separated key-value pairs, where key is the cluster algorithm
  -- and value is the cluster id.
  -- key can be repeated.
  -- The clusters the first test result of the verdict is in.
  -- Once the test result reaches its retention period in the clustering
  -- system, this will cease to be updated.
  Clusters ARRAY<STRING(MAX)>,

) PRIMARY KEY (Realm, TestId, VariantHash, InvocationId),
INTERLEAVE IN PARENT AnalyzedTestVariants ON DELETE CASCADE;

-- Used by finding most recent verdicts of a test variant to calculate status.
CREATE NULL_FILTERED INDEX VerdictsByTestVariantAndIngestionTime
 ON Verdicts (Realm, TestId, VariantHash, IngestionTime DESC),
 INTERLEAVE IN AnalyzedTestVariants;

-- FailureAssociationRules associate failures with bugs. When a rule
-- is used to match incoming test failures, the resultant cluster is
-- known as a 'bug cluster' because the failures in it are associated
-- with a bug (via the failure association rule).
-- The ID of a bug cluster corresponding to a rule is
-- (Project, RuleBasedClusteringAlgorithm, RuleID), where
-- RuleBasedClusteringAlgorithm is the algorithm name of the algorithm
-- that clusters failures based on failure association rules (e.g.
-- 'rules-v2'), and (Project, RuleId) is the ID of the rule.
CREATE TABLE FailureAssociationRules (
  -- The LUCI Project this bug belongs to.
  Project STRING(40) NOT NULL,
  -- The unique identifier for the rule. This is a randomly generated
  -- 128-bit ID, encoded as 32 lowercase hexadecimal characters.
  RuleId STRING(32) NOT NULL,
  -- The rule predicate, defining which failures are being associated.
  RuleDefinition STRING(65536) NOT NULL,
  -- The time the rule was created.
  CreationTime TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
  -- The user which created the rule. If this was auto-filed by LUCI Analysis
  -- itself, this is the special value 'system'. Otherwise, it is
  -- an email address.
  -- 320 is the maximum length of an email address (64 for local part,
  -- 1 for the '@', and 255 for the domain part).
  CreationUser STRING(320) NOT NULL,
  -- The last time the rule was updated.
  LastUpdated TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
  -- The user which last updated this rule. If this was LUCI Analysis itself,
  -- (e.g. in case of an auto-filed bug which was created and never
  -- modified) this is 'system'. Otherwise, it is an email address.
  LastUpdatedUser STRING(320) NOT NULL,
  -- The time the rule was last updated in a way that caused the
  -- matched failures to change, i.e. because of a change to RuleDefinition
  -- or IsActive. (For comparison, updating BugID does NOT change
  -- the matched failures, so does NOT update this field.)
  -- When this value changes, it triggers re-clustering.
  -- Basis for RulesVersion on ClusteringState and ReclusteringRuns.
  PredicateLastUpdated TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
  -- The bug the failures are associated with (part 1). This is the
  -- bug tracking system containing the bug the failures are associated
  -- with. The only supported values are 'monorail' and 'buganizer'.
  BugSystem STRING(16) NOT NULL,
  -- The bug the failures are associated with (part 2). This is the
  -- identifier of the bug the failures are associated with, as identified
  -- by the bug tracking system itself. For monorail, the scheme is
  -- {project}/{numeric_id}, for buganizer, the scheme is {numeric_id}.
  BugId STRING(255) NOT NULL,
  -- Whether the bug must still be updated by LUCI Analysis, and whether
  -- failures should still be matched against this rule. The only allowed
  -- values are true or NULL (to indicate false). Only if the bug has
  -- been closed and no failures have been observed for a while should
  -- this be NULL. This makes it easy to retrofit a NULL_FILTERED index
  -- in future, if it is needed for performance.
  IsActive BOOL,
  -- Whether this rule should manage the priority and verified status
  -- of the associated bug based on the impact of the cluster defined
  -- by this rule.
  -- The only allowed values are true or NULL (to indicate false).
  IsManagingBug BOOL,
  -- Indicates whether we should update the priority of the bug
  -- that is associated with this rule based on the cluster's impact.
  -- This toggle has must have no effect if the IsManagingBug toggle is off.
  -- This value should not be null.
  -- This column has a default value of TRUE in prod, but it is commented
  -- out here because spanner emulator doesn't support DEFAULT
  -- expressions, see https://github.com/GoogleCloudPlatform/cloud-spanner-emulator/issues/71.
  IsManagingBugPriority BOOL NOT NULL, -- DEFAULT TRUE,

  -- Tracks when the field IsManagingBugPriority was last updated.
  IsManagingBugPriorityLastUpdated TIMESTAMP OPTIONS (allow_commit_timestamp=true),
  -- The suggested cluster this failure association rule was created from
  -- (if any) (part 1).
  -- This is the algorithm component of the suggested cluster this rule
  -- was created from.
  -- Until re-clustering is complete (and the residual impact of the source
  -- cluster has reduced to zero), SourceClusterAlgorithm and SourceClusterId
  -- tell bug filing to ignore the source suggested cluster when
  -- determining whether new bugs need to be filed.
  SourceClusterAlgorithm STRING(32) NOT NULL,
  -- The suggested cluster this failure association rule was created from
  -- (if any) (part 2).
  -- This is the algorithm-specific ID component of the suggested cluster
  -- this rule was created from.
  SourceClusterId STRING(32) NOT NULL,
) PRIMARY KEY (Project, RuleId);

-- The failure association rules associated with a bug. This also
-- enforces the constraint that there is at most one rule per bug
-- per project.
CREATE UNIQUE INDEX FailureAssociationRuleByBugAndProject ON FailureAssociationRules(BugSystem, BugId, Project);

-- Enforces the constraint that only one rule may manage a given bug
-- at once.
-- This is required to ensure that automatic bug filing does not attempt to
-- take conflicting actions (i.e. simultaneously increase and decrease
-- priority) on the same bug, because of differing priorities set by
-- different rules.
CREATE UNIQUE NULL_FILTERED INDEX FailureAssociationRuleByManagedBug ON FailureAssociationRules(BugSystem, BugId, IsManagingBug);

-- Clustering state records the clustering state of failed test results, organised
-- by chunk.
CREATE TABLE ClusteringState (
  -- The LUCI Project the test results belong to.
  Project STRING(40) NOT NULL,
  -- The identity of the chunk of test results. 32 lowercase hexadecimal
  -- characters assigned by the ingestion process.
  ChunkId STRING(32) NOT NULL,
  -- The start of the retention period of the test results in the chunk.
  PartitionTime TIMESTAMP NOT NULL,
  -- The identity of the blob storing the chunk's test results.
  ObjectId STRING(32) NOT NULL,
  -- The version of clustering algorithms used to cluster test results in this
  -- chunk. (This is a version over the set of algorithms, distinct from the
  -- version of a single algorithm, e.g.:
  -- v1 -> {reason-v1}, v2 -> {reason-v1, testname-v1},
  -- v3 -> {reason-v2, testname-v1}.)
  AlgorithmsVersion INT64 NOT NULL,
  -- The version of project configuration used by algorithms to match test
  -- results in this chunk.
  ConfigVersion TIMESTAMP NOT NULL,
  -- The version of the set of failure association rules used to match test
  -- results in this chunk. This is the maximum "Predicate Last Updated" time
  -- of any failure association rule in the snapshot of failure association
  -- rules used to match the test results.
  RulesVersion TIMESTAMP NOT NULL,
  -- Serialized ChunkClusters proto containing which test result is in which
  -- cluster.
  Clusters BYTES(MAX) NOT NULL,
  -- The Spanner commit timestamp of when the row was last updated.
  LastUpdated TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
) PRIMARY KEY (Project, ChunkId)
, ROW DELETION POLICY (OLDER_THAN(PartitionTime, INTERVAL 90 DAY));

-- ReclusteringRuns contains details of runs used to re-cluster test results.
CREATE TABLE ReclusteringRuns (
  -- The LUCI Project.
  Project STRING(40) NOT NULL,
  -- The attempt. This is the timestamp the orchestrator run ends.
  AttemptTimestamp TIMESTAMP NOT NULL,
  -- The minimum algorithms version the reclustering run is trying to achieve.
  -- Chunks with an AlgorithmsVersion less than this value are eligible to be
  -- re-clustered.
  AlgorithmsVersion INT64 NOT NULL,
  -- The minimum config version the reclustering run is trying to achieve.
  -- Chunks with a ConfigVersion less than this value are eligible to be
  -- re-clustered.
  ConfigVersion TIMESTAMP NOT NULL,
  -- The minimum rules version the reclustering run is trying to achieve.
  -- Chunks with a RulesVersion less than this value are eligible to be
  -- re-clustered.
  RulesVersion TIMESTAMP NOT NULL,
  -- The number of shards created for this run (for this LUCI project).
  ShardCount INT64 NOT NULL,
  -- The number of shards that have reported progress (at least once).
  -- When this is equal to ShardCount, readers can have confidence Progress
  -- is a reasonable reflection of the progress made reclustering
  -- this project. Until then, it is a loose lower-bound.
  ShardsReported INT64 NOT NULL,
  -- The progress. This is a value between 0 and 1000*ShardCount.
  Progress INT64 NOT NULL,
) PRIMARY KEY (Project, AttemptTimestamp DESC)
, ROW DELETION POLICY (OLDER_THAN(AttemptTimestamp, INTERVAL 90 DAY));

-- ReclusteringShards is used to for shards to report progress re-clustering
-- test results. Generally only progress for the most recent minute will
-- be in this table.
CREATE TABLE ReclusteringShards (
  -- A unique number for the shard. Shards are numbered sequentially,
  -- starting from one.
  ShardNumber INT64 NOT NULL,
  -- The attempt. This is the timestamp the orchestrator run ends.
  AttemptTimestamp TIMESTAMP NOT NULL,
  -- The LUCI Project the shard is doing reclustering for.
  Project STRING(40) NOT NULL,
  -- The progress. This is a value between 0 and 1000. If this is NULL,
  -- it means progress has not yet been reported by the shard.
  Progress INT64,
) PRIMARY KEY (ShardNumber, AttemptTimestamp DESC);

-- Ingestions is used to synchronise and deduplicate the ingestion
-- of test results which require data from one or more sources.
--
-- Ingestion may only start after the following events are received:
-- 1. The build has completed.
-- 2. The invocation containing its test results (if any)
--    has been finalized.
-- 3. The presubmit run (if any) has completed.
-- These events may occur in any order (e.g. 3 can occur before 1 if the
-- presubmit run fails before all builds are complete).
CREATE TABLE Ingestions (
  -- The unique key for the ingestion. The current scheme is:
  -- {buildbucket host name}/{build id}.
  BuildId STRING(1024) NOT NULL,
  -- The LUCI Project to which the build belongs. Populated at the same
  -- time as the build result.
  BuildProject STRING(40),
  -- The build result.
  BuildResult BYTES(MAX),
  -- The Spanner commit time the build result was populated.
  BuildJoinedTime TIMESTAMP OPTIONS (allow_commit_timestamp=true),
  -- Does the build have a ResultDB invocation? If yes, then ingestion should
  -- wait for the invocation result to be populated before commencing ingestion.
  -- (In practice, ingestion of a build without an invocation does nothing, but
  -- we schedule an ingestion for it anyway as for monitoring purposes it is
  -- convenient if all builds yield an ingestion task.)
  -- Only populated once either the BuildResult or InvocationResult has been set.
  HasInvocation BOOL,
  -- The LUCI Project to which the invocation belongs. Populated at the same
  -- time as the invocation result.
  InvocationProject STRING(40),
  -- The invocation result.
  InvocationResult BYTES(MAX),
  -- The Spanner commit time the invocation result was populated.
  InvocationJoinedTime TIMESTAMP OPTIONS (allow_commit_timestamp=true),
  -- Is the build part of a presubmit run? If yes, then ingestion should
  -- wait for the presubmit result to be populated before commencing ingestion.
  -- Use 'true' to indicate true and NULL to indicate false.
  -- Only populated once either the BuildResult or PresubmitResult has been set.
  IsPresubmit BOOL,
  -- The LUCI Project to which the presubmit run belongs. Populated at the
  -- same time as the presubmit run result.
  PresubmitProject STRING(40),
  -- The presubmit result.
  PresubmitResult BYTES(MAX),
  -- The Spanner commit time the presubmit result was populated.
  PresubmitJoinedTime TIMESTAMP OPTIONS (allow_commit_timestamp=true),
  -- The Spanner commit time the row last last updated.
  LastUpdated TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
  -- The number of test result ingestion tasks have been created for this
  -- invocation.
  -- Used to avoid duplicate scheduling of ingestion tasks. If the page_index
  -- is the index of the page being processed, an ingestion task for the next
  -- page will only be created if (page_index + 1) == TaskCount.
  TaskCount INT64,
) PRIMARY KEY (BuildId)
-- 90 days retention, plus some margin (10 days) to ensure ingestion records
-- are always retained longer than the ingested results (acknowledging
-- the partition time on ingested chunks may be later than the LastUpdated
-- time if clocks are not synchronised).
, ROW DELETION POLICY (OLDER_THAN(LastUpdated, INTERVAL 100 DAY));

-- Stores transactional tasks reminders.
-- See https://go.chromium.org/luci/server/tq. Scanned by tq-sweeper-spanner.
CREATE TABLE TQReminders (
    ID STRING(MAX) NOT NULL,
    FreshUntil TIMESTAMP NOT NULL,
    Payload BYTES(102400) NOT NULL,
) PRIMARY KEY (ID ASC);

CREATE TABLE TQLeases (
    SectionID STRING(MAX) NOT NULL,
    LeaseID INT64 NOT NULL,
    SerializedParts ARRAY<STRING(MAX)>,
    ExpiresAt TIMESTAMP NOT NULL,
) PRIMARY KEY (SectionID ASC, LeaseID ASC);

-- Stores test results.
-- As of Q2 2022, this table is estimated to collect ~250 billion rows over
-- 90 days. Please be mindful of storage implications when adding new fields.
-- https://cloud.google.com/spanner/docs/reference/standard-sql/data-types#storage_size_for_data_types
-- gives guidance on the storage sizes of data types.
CREATE TABLE TestResults (
  -- The LUCI Project this test result belongs to.
  Project STRING(40) NOT NULL,

  -- Unique identifier of the test.
  -- This has the same value as luci.resultdb.v1.TestResult.test_id.
  TestId STRING(MAX) NOT NULL,

  -- Partition time, as determined by LUCI Analysis ingestion.
  -- Start time of the ingested build (for postsubmit results) or start time
  -- of the presubmit run (for presubmit results). Defines date/time axis of
  -- test results plotted by date/time, and start of data retention period.
  -- Including as part of Primary Key allows direct filtering of data for test
  -- to last N days. This could be used to improve performance for tests with
  -- many results, or allow experimentation with keeping longer histories
  -- (e.g. 120 days) without incurring performance penalty on time-windowed
  -- queries.
  PartitionTime TIMESTAMP NOT NULL,

  -- A hex-encoded sha256 of concatenated "<key>:<value>\n" variant pairs.
  -- Computed as hex(sha256(<concatenated_key_value_pairs>)[:8]),
  -- where concatenated_key_value_pairs is the result of concatenating
  -- variant pairs formatted as "<key>:<value>\n" in ascending key order.
  -- Combination of Realm, TestId and VariantHash can identify a test variant.
  VariantHash STRING(16) NOT NULL,

  -- The invocation from which these test results were ingested.
  -- This is the top-level invocation that was ingested.
  IngestedInvocationId STRING(MAX) NOT NULL,

  -- The index of the test run that contained this test result.
  -- The test run of a test result is the invocation it is directly
  -- included inside; typically the invocation for the swarming task
  -- the tests ran as part of.
  -- Indexes are assigned to runs based on the order they appear to have
  -- run in, starting from zero (based on test result timestamps).
  -- However, if two test runs overlap, the order of indexes for those test
  -- runs is not guaranteed.
  RunIndex INT64 NOT NULL,

  -- The index of the test result in the run. The first test result that
  -- was produced in a run will have index 0, the second will have index 1,
  -- and so on.
  ResultIndex INT64 NOT NULL,

  -- Whether the test result was expected.
  -- The value 'true' is used to encode true, and NULL encodes false.
  IsUnexpected BOOL,

  -- How long the test execution took, in microseconds.
  RunDurationUsec INT64,

  -- The test result status.
  Status INT64 NOT NULL,

  -- The reasons (if any) the test verdict was exonerated.
  -- If this array is null, the test verdict was not exonerated.
  -- (Non-null) empty array values are not used.
  -- This field is stored denormalised. It is guaranteed to be the same for
  -- all results for a test variant in an ingested invocation.
  ExonerationReasons ARRAY<INT64>,

  -- The following data is stored denormalised. It is guaranteed to be
  -- the same for all results for the ingested invocation.

  -- The realm of the test result, excluding project. 62 as ResultDB allows
  -- at most 64 characters for the construction "<project>:<realm>" and project
  -- must be at least one character.
  SubRealm STRING(62) NOT NULL,

  -- The status of the build that contained this test result. Can be used
  -- to filter incomplete results (e.g. where build was cancelled or had
  -- an infra failure).
  -- See luci.analysis.v1.BuildStatus.
  BuildStatus INT64 NOT NULL,

  -- The run mode of the presubmit run (e.g. DRY RUN, FULL RUN).
  -- Only populated for builds part of presubmit runs.
  PresubmitRunMode INT64,

  -- The identity of the git reference defining the code line that was tested.
  -- This excludes any unsubmitted changes that were tested, which are
  -- noted separately in the Changelist... fields below.
  --
  -- The details of the git reference is stored in the GitReferences table,
  -- keyed by (Project, GitReferenceHash).
  --
  -- Only populated if CommitPosition is populated.
  GitReferenceHash BYTES(8),

  -- The commit position along the given git reference that was tested.
  -- This excludes any unsubmitted changes that were tested, which are
  -- noted separately in the Changelist... fields below.
  -- This is populated from the buildbucket build outputs or inputs, usually
  -- as calculated via goto.google.com/git-numberer.
  --
  -- Only populated if build reports the commit position as part of the
  -- build outputs or inputs.
  CommitPosition INT64,

  -- The following fields capture information about any unsubmitted
  -- changelists that were tested by the test execution. The arrays
  -- are matched in length and correspond in index, i.e.
  -- ChangelistHosts[OFFSET(0)] corresponds with ChangelistChanges[OFFSET(0)]
  -- and ChangelistPatchsets[OFFSET(0)],
  -- with the exception of ChangelistOwnerKinds, for which correspondance
  -- is not guaranteed until March 2023 (as the column was retrofitted later).
  --
  -- Changelists are stored in ascending lexicographical order (over
  -- (hostname, change, patchset)).
  -- They will be set for all presubmit runs, and may be set for other
  -- builds as well (even those outside a formal LUCI CV run) based on
  -- buildbucket inputs. At most 10 changelists are included.

  -- Hostname(s) of the gerrit instance of the changelist that was tested
  -- (if any). For storage efficiency, the suffix "-review.googlesource.com"
  -- is not stored if it is present. When reading, if the value read
  -- does not contain dots ('.'), the suffix should be added back.
  -- Otherwise, the value can be assumed to be complete.
  ChangelistHosts ARRAY<STRING(255)> NOT NULL,

  -- The changelist number(s), e.g. 12345.
  ChangelistChanges ARRAY<INT64> NOT NULL,

  -- The patchset number(s) of the changelist, e.g. 1.
  ChangelistPatchsets ARRAY<INT64> NOT NULL,

  -- The changelist owner kind(s). Elements in this array correspond to
  -- one of the luci.analysis.v1.ChangelistOwnerKinds values.
  -- 'U' corresponds to a User changelist, 'A' corresponds to an Automation
  -- changelist, and '' corresponds to a changelist of unspecified origin.
  ChangelistOwnerKinds ARRAY<STRING(1)>,
) PRIMARY KEY(Project, TestId, PartitionTime DESC, VariantHash, IngestedInvocationId, RunIndex, ResultIndex)
, ROW DELETION POLICY (OLDER_THAN(PartitionTime, INTERVAL 90 DAY));

-- Stores git references. Git references represent a linear source code
-- history along which the position of commits can be measured
-- using an integer (where larger integer means later in history and
-- smaller integer means earlier in history).
CREATE TABLE GitReferences (
  -- The LUCI Project this git reference was used in.
  -- Although the same git reference could be used in different projects,
  -- it is stored namespaced by project to isolate projects from each other.
  Project STRING(40) NOT NULL,

  -- The identity of the git reference.
  -- Constructed by hashing the following values:
  -- - The gittiles hostname, e.g. "chromium.googlesource.com".
  -- - The repository name, e.g. "chromium/src".
  -- - The reference name, e.g. "refs/heads/main".
  -- Using the following formula ([:8] indicates truncation to 8 bytes).
  -- SHA256(hostname + "\n" + repository_name + "\n"  + ref_name)[:8].
  GitReferenceHash BYTES(8) NOT NULL,

  -- The gittiles hostname. E.g. "chromium.googlesource.com".
  -- 255 characters for max length of a domain name.
  Hostname STRING(255) NOT NULL,

  -- The gittiles repository name (also known as the gittiles "project").
  -- E.g. "chromium/src".
  -- 4096 for the maximum length of a linux path.
  Repository STRING(4096) NOT NULL,

  -- The git reference name, e.g. "refs/heads/main".
  Reference STRING(4096) NOT NULL,

  -- Last (ingestion) time this git reference was observed.
  -- This value may be out of date by up to 24 hours to allow for contention-
  -- reducing strategies.
  LastIngestionTime TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
) PRIMARY KEY(Project, GitReferenceHash)
-- Use a slightly longer retention period to prevent the git reference being
-- dropped before the associated TestResults.
, ROW DELETION POLICY (OLDER_THAN(LastIngestionTime, INTERVAL 100 DAY));

-- Stores top-level invocations which were ingested.
--
-- TODO(crbug.com/1266759):
-- This forms part of an experiment embedded into the design.
-- If joining to this table is efficient, we may leave Changelist,
-- Build Status, realm and commit position data here and drop it
-- off the TestResults table.
-- If not, we may decide to delete this table.
CREATE TABLE IngestedInvocations (
  -- The LUCI Project the invocation is a part of.
  Project STRING(40) NOT NULL,

  -- The (top-level) invocation which was ingested.
  IngestedInvocationId STRING(MAX) NOT NULL,

  -- The realm of the invocation, excluding project. 62 as ResultDB allows
  -- at most 64 characters for the construction "<project>:<realm>" and project
  -- must be at least one character.
  SubRealm STRING(62) NOT NULL,

  -- Partition time, as determined by LUCI Analysis ingestion. Start time of
  -- the ingested build (for postsubmit results) or start time of the
  -- presubmit run (for presubmit results).
  PartitionTime TIMESTAMP NOT NULL,

  -- The status of the build that contained this test result. Can be used
  -- to filter incomplete results (e.g. where build was cancelled or had
  -- an infra failure).
  -- See luci.analysis.v1.BuildStatus.
  BuildStatus INT64,

  -- The owner of the presubmit run.
  -- This owner of the CL on which CQ+1/CQ+2 was clicked
  -- (even in case of presubmit run with multiple CLs).
  -- There is scope for this field to become an email address if privacy
  -- approval is obtained, until then it is "automation" (for automation
  -- service accounts) and "user" otherwise.
  -- Only populated for builds part of presubmit runs.
  PresubmitRunOwner STRING(320),

  -- The run mode of the presubmit run (e.g. DRY RUN, FULL RUN).
  -- Only populated for builds part of presubmit runs.
  PresubmitRunMode INT64,


  -- The identity of the git reference defining the code line that was tested.
  -- This excludes any unsubmitted changes that were tested, which are
  -- noted separately in the Changelist... fields below.
  --
  -- The details of the git reference is stored in the GitReferences table,
  -- keyed by (Project, GitReferenceHash).
  --
  -- Only populated if CommitPosition is populated.
  GitReferenceHash BYTES(8),

  -- The commit position along the given git reference that was tested.
  -- This excludes any unsubmitted changes that were tested, which are
  -- noted separately in the Changelist... fields below.
  -- This is populated from the buildbucket build outputs or inputs, usually
  -- as calculated via goto.google.com/git-numberer.
  --
  -- Only populated if build reports the commit position as part of the
  -- build outputs or inputs.
  CommitPosition INT64,

  -- The SHA-1 commit hash of the commit that was tested.
  -- Encoded as a lowercase hexadecimal string.
  -- This excludes any unsubmitted changes that were tested, which are
  -- noted separately in the Changelist... fields below.
  --
  -- Only populated if CommitPosition is populated.
  CommitHash STRING(40),

  -- The following fields capture information about any unsubmitted
  -- changelists that were tested by the test execution. The arrays
  -- are matched in length and correspond in index, i.e.
  -- ChangelistHosts[OFFSET(0)] corresponds with ChangelistChanges[OFFSET(0)]
  -- and ChangelistPatchsets[OFFSET(0)],
  -- with the exception of ChangelistOwnerKinds, for which correspondance
  -- is not guaranteed until March 2023 (as the column was retrofitted later).
  --
  -- Changelists are stored in ascending lexicographical order (over
  -- (hostname, change, patchset)).
  -- They will be set for all presubmit runs, and may be set for other
  -- builds as well (even those outside a formal LUCI CV run) based on
  -- buildbucket inputs. At most 10 changelists are included.

  -- Hostname(s) of the gerrit instance of the changelist that was tested
  -- (if any). For storage efficiency, the suffix "-review.googlesource.com"
  -- is not stored if it is present. When reading, if the value read
  -- does not contain dots ('.'), the suffix should be added back.
  -- Otherwise, the value can be assumed to be complete.
  ChangelistHosts ARRAY<STRING(255)> NOT NULL,

  -- The changelist number(s), e.g. 12345.
  ChangelistChanges ARRAY<INT64> NOT NULL,

  -- The patchset number(s) of the changelist, e.g. 1.
  ChangelistPatchsets ARRAY<INT64> NOT NULL,

  -- The changelist owner kind(s). Elements in this array correspond to
  -- one of the luci.analysis.v1.ChangelistOwnerKinds values.
  -- 'U' corresponds to a User changelist, 'A' corresponds to an Automation
  -- changelist, and '' corresponds to a changelist of unspecified origin.
  ChangelistOwnerKinds ARRAY<STRING(1)>,
) PRIMARY KEY(Project, IngestedInvocationId)
-- Use a slightly longer retention period to prevent the invocation being
-- dropped before the associated TestResults.
, ROW DELETION POLICY (OLDER_THAN(PartitionTime, INTERVAL 100 DAY));

-- Serves two purposes:
-- - Permits listing of distinct variants observed for a test in a project,
--   filtered by Realm.
--
-- - Provides a mapping back from VariantHash to variant.
--
-- TODO(crbug.com/1266759):
-- UniqueTestVariants table in ResultDB will be superseded by this table and
-- will need to be deleted.
CREATE TABLE TestVariantRealms (
  -- The LUCI Project in which the variant was observed.
  Project STRING(40) NOT NULL,

  -- Unique identifier of the test from which the variant was observed,
  -- This has the same value as luci.resultdb.v1.TestResult.test_id.
  TestId STRING(MAX) NOT NULL,

  -- A hex-encoded sha256 of concatenated "<key>:<value>\n" variant pairs.
  -- Computed as hex(sha256(<concatenated_key_value_pairs>)[:8]),
  -- where concatenated_key_value_pairs is the result of concatenating
  -- variant pairs formatted as "<key>:<value>\n" in ascending key order.
  -- Combination of Realm, TestId and VariantHash can identify a test variant.
  VariantHash STRING(16) NOT NULL,

  -- The realm of the test result from which the variant was observed, excluding
  -- project. 62 as ResultDB allows at most 64 characters for the construction
  -- "<project>:<realm>" and project must be at least one character.
  SubRealm STRING(62) NOT NULL,

  -- key:value pairs to specify the way of running the test.
  -- See also luci.resultdb.v1.TestResult.variant.
  Variant ARRAY<STRING(MAX)>,

  -- Other information about the test variant, like information from tags,
  -- could be captured here, as is currently the case for AnalyzedTestVariants.
  -- (e.g. test ownership).

  -- Last (ingestion) time this test variant was observed in the realm.
  -- This value may be out of date by up to 24 hours to allow for contention-
  -- reducing strategies.
  LastIngestionTime TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
) PRIMARY KEY(Project, TestId, VariantHash, SubRealm)
-- Use a slightly longer retention period to prevent the invocation being
-- dropped before the associated TestResults.
, ROW DELETION POLICY (OLDER_THAN(LastIngestionTime, INTERVAL 100 DAY));

-- Permits listing of distinct tests observed for a project, filtered by Realm.
-- This table is created to support test ID substring search, which can often
-- lead to a full table scan, which will be significantly slower in the
-- TestVariantRealms table.
CREATE TABLE TestRealms (
  -- The LUCI Project in which the variant was observed.
  Project STRING(40) NOT NULL,

  -- Unique identifier of the test from which the variant was observed,
  -- This has the same value as luci.resultdb.v1.TestResult.test_id.
  TestId STRING(MAX) NOT NULL,

  -- The realm of the test result from which the variant was observed, excluding
  -- project. 62 as ResultDB allows at most 64 characters for the construction
  -- "<project>:<realm>" and project must be at least one character.
  SubRealm STRING(62) NOT NULL,

  -- Last (ingestion) time this test variant was observed in the realm.
  -- This value may be out of date by up to 24 hours to allow for contention-
  -- reducing strategies.
  LastIngestionTime TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
) PRIMARY KEY(Project, TestId, SubRealm)
-- Use a slightly longer retention period to prevent the invocation being
-- dropped before the associated TestResults.
, ROW DELETION POLICY (OLDER_THAN(LastIngestionTime, INTERVAL 100 DAY));

-- Uses in test variant analysis (see go/luci-test-variant-analysis-design).
-- Stores information about (test, variant, branch) combination.
-- The information stored is the verdict history, and the analyzed segment
-- results.
CREATE TABLE TestVariantBranch (
  -- The LUCI Project.
  Project STRING(40) NOT NULL,
  -- Unique identifier for the test.
  TestId STRING(MAX) NOT NULL,
  -- The identify of the test variant.
  -- Computed as hex(sha256(<concatenated_key_value_pairs>)[:8]),
  -- where concatenated_key_value_pairs is the result of concatenating
  -- variant pairs formatted as "<key>:<value>\n" in ascending key order.
  VariantHash STRING(16) NOT NULL,
  -- The identity of the branch that was tested.
  RefHash BYTES(8) NOT NULL,
  -- key:value pairs in the test variant. See also Variant on the ResultDB
  -- TestResults table. Only written the first time the row is created.
  Variant ARRAY<STRING(MAX)>,
  -- ZStandard-compressed, serialized luci.analysis.v1.SourceRef.
  -- SourceRef represents a reference in a source control system.
  -- Only written the first time the row is created.
  SourceRef BYTES(MAX) NOT NULL,
  -- ZStandard-compressed representation of up to 100 recent test verdicts for
  --  the test variant.
  HotInputBuffer BYTES(MAX) NOT NULL,
  -- ZStandard-compressed representation of up to 2000 recent test verdicts for
  --  the test variant, after those in HotInputBuffer. Verdicts in
  -- HotInputBuffer are pushed here when HotInputBuffer is full.
  ColdInputBuffer BYTES(MAX) NOT NULL,
  -- ZStandard-compressed, serialized Segment proto describing the finalizing
  -- segment (if any). When verdicts are evicted from the ColdInputBuffer,
  -- their totals are added to this segment.
  FinalizingSegment BYTES(MAX) NOT NULL,
  -- ZStandard-compressed, serialized Segments proto describing the finalized
  -- segments only.
  -- We only store up to 100 finalized segments.
  FinalizedSegments BYTES(MAX) NOT NULL,
  -- The Spanner commit timestamp this row was last updated.
  -- Used as version timestamp for BigQuery export.
  LastUpdated TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
) PRIMARY KEY(Project, TestId, VariantHash, RefHash),
  ROW DELETION POLICY (OLDER_THAN(LastUpdated, INTERVAL 90 DAY));

-- This table is to ensure that the ingestion to the TestVariantBranch table
-- only happens once, even in the case of task retry.
-- This table will store the batches that have been processed by change point
-- analysis.
-- Insertions to this table will happen in the same transaction as
-- insertions to the TestVariantBranch table.
-- Before processing a batch of test variants in change point analysis, we will
-- first check if the batch exists in this table, if yes, then the batch has
-- been processed and we should skip it.
-- The retention is set to 31 days, since it is the maximum time a task can
-- live in task queue is 31 days (https://cloud.google.com/tasks/docs/quotas).
CREATE TABLE TestVariantBranchCheckpoint (
  -- The invocation id of the batch.
  InvocationId STRING(MAX) NOT NULL,
  -- The test ID of the first test variant in the batch.
  StartingTestId STRING(MAX) NOT NULL,
  -- The variant hash of the first test variant in the batch.
  StartingVariantHash STRING(16) NOT NULL,
  -- Time that this record was inserted in the table.
  InsertionTime TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
) PRIMARY KEY(InvocationId, StartingTestId, StartingVariantHash),
  ROW DELETION POLICY (OLDER_THAN(InsertionTime, INTERVAL 31 DAY));

-- This table is to determine if a run is a recycled run for test variant
-- analysis.
CREATE TABLE Invocations (
  -- The LUCI Project of the Ingested Invocation. Although Invocation IDs
  -- are unique on their own, this enforces hard separation between the data
  -- of each project. It ensures the same test results can be ingested into two
  -- different LUCI Projects, but not the same project.
  Project STRING(40) NOT NULL,
  -- The invocation of the test run.
  InvocationID STRING(MAX) NOT NULL,
  -- The top-level invocation the test run will be ingested as part of.
  -- If it is seen in another invocation ID, it will be considered a
  -- duplicate test run.
  IngestedInvocationID STRING(MAX) NOT NULL,
  -- Used to enforce a deletion policy on this data.
  CreationTime TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
) PRIMARY KEY(Project, InvocationID),
  ROW DELETION POLICY (OLDER_THAN(CreationTime, INTERVAL 91 DAY));
