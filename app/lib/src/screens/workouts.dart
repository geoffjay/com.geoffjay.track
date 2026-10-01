/// Workouts tab: workout log with entry summaries, create/edit via an
/// entry builder (weighted: weight × reps; timed: duration + optional
/// distance), finish an active session, and the exercise catalog.
library;

import 'package:flutter/material.dart';

import '../api_client.dart';
import '../app_state.dart';
import '../models.dart';
import '../widgets.dart';

class WorkoutsScreen extends StatefulWidget {
  const WorkoutsScreen({super.key});

  @override
  State<WorkoutsScreen> createState() => _WorkoutsScreenState();
}

class _WorkoutsScreenState extends State<WorkoutsScreen> {
  int _epoch = 0;

  ApiClient get _api => AppStateScope.of(context).client!;

  void _refresh() => setState(() => _epoch++);

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Workouts'),
        actions: [
          IconButton(
            icon: const Icon(Icons.fitness_center),
            tooltip: 'Exercise catalog',
            onPressed: () => Navigator.of(context).push(MaterialPageRoute(
                builder: (_) => const ExerciseCatalogScreen())).then((_) => _refresh()),
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => _form(null),
        icon: const Icon(Icons.add),
        label: const Text('Log workout'),
      ),
      body: LoadableList<Workout>(
        key: ValueKey('workouts-$_epoch'),
        fetch: _api.listWorkouts,
        emptyMessage:
            'No workouts yet.\nTap Log workout to record a session.',
        builder: (workouts, _) => ListView.builder(
          padding: const EdgeInsets.only(bottom: 88),
          itemCount: workouts.length,
          itemBuilder: (context, i) =>
              _workoutTile(context, workouts[i]),
        ),
      ),
    );
  }

  Widget _workoutTile(BuildContext context, Workout w) {

    final title = w.name.isNotEmpty ? w.name : 'Workout';
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      child: Column(
        children: [
          ListTile(
            leading: w.isFinished
                ? const Icon(Icons.check_circle_outline)
                : const Icon(Icons.play_circle_outline),
            title: Text(title),
            subtitle: Text(
              '${fmtDateTime(w.startedAt)}'
              '${w.durationMin != null ? ' · ${fmtDuration(w.durationMin! / 60)}' : ' · in progress'}'
              '${w.effort != null ? ' · effort ${w.effort}/10' : ''}',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            trailing: PopupMenuButton<String>(
              onSelected: (action) {
                switch (action) {
                  case 'edit':
                    _form(w);
                  case 'finish':
                    _finish(w);
                  case 'delete':
                    _delete(w);
                }
              },
              itemBuilder: (_) => [
                const PopupMenuItem(
                    value: 'edit', child: Text('Edit')),
                if (!w.isFinished)
                  const PopupMenuItem(
                      value: 'finish', child: Text('Finish')),
                const PopupMenuItem(
                    value: 'delete', child: Text('Delete')),
              ],
            ),
          ),
          if (w.entries.isNotEmpty)
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 0, 16, 10),
              child: Column(
                children: [
                  for (final (j, e) in w.entries.indexed)
                    Padding(
                      padding: EdgeInsets.only(top: j == 0 ? 0 : 4),
                      child: Row(
                        children: [
                          Expanded(
                            flex: 3,
                            child: Text(e.exerciseName,
                                overflow: TextOverflow.ellipsis),
                          ),
                          Expanded(
                            flex: 2,
                            child: Text(entrySummary(e),
                                textAlign: TextAlign.end),
                          ),
                        ],
                      ),
                    ),
                ],
              ),
            ),
        ],
      ),
    );
  }

  Future<void> _form(Workout? existing) async {
    final result = await _workoutForm(context, existing);
    if (result == null) return;
    try {
      if (existing == null) {
        await _api.createWorkout(
          name: result.$1,
          notes: result.$2,
          effort: result.$3,
          entries: result.$4,
        );
        if (mounted) showSnack(context, 'Workout logged');
      } else {
        await _api.updateWorkout(
          existing.id,
          name: result.$1,
          notes: result.$2,
          effort: result.$3,
          entries: result.$4,
        );
        if (mounted) showSnack(context, 'Workout updated');
      }
    } on ApiException catch (e) {
      if (mounted) showSnack(context, e.message, error: true);
    }
    _refresh();
  }

  Future<void> _finish(Workout w) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Finish workout?'),
        content: Text(
            'Started ${fmtDateTime(w.startedAt)} — the end time will be now.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('Cancel')),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('Finish'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await _api.finishWorkout(w.id);
      if (mounted) showSnack(context, 'Workout finished');
    } on ApiException catch (e) {
      if (mounted) showSnack(context, e.message, error: true);
    }
    _refresh();
  }

  Future<void> _delete(Workout w) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Delete workout?'),
        content: Text('Started ${fmtDateTime(w.startedAt)}.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('Cancel')),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            style: FilledButton.styleFrom(
                backgroundColor: Theme.of(dialogContext).colorScheme.error),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await _api.deleteWorkout(w.id);
      if (mounted) showSnack(context, 'Deleted');
    } on ApiException catch (e) {
      if (mounted) showSnack(context, e.message, error: true);
    }
    _refresh();
  }
}

/// One entry being drafted in the workout form. The shape follows the
/// exercise's type: weighted needs weight + reps; timed needs duration.
class _EntryDraft {
  final int exerciseId;
  final String exerciseName;
  final String exerciseType;
  final double? weightKg;
  final int? reps;
  final double? durationSec;
  final double? distanceKm;
  final String notes;

  const _EntryDraft({
    required this.exerciseId,
    required this.exerciseName,
    required this.exerciseType,
    this.weightKg,
    this.reps,
    this.durationSec,
    this.distanceKm,
    this.notes = '',
  });

  String get summary => exerciseType == 'weighted'
      ? '${weightKg == null ? '—' : fmtValue(weightKg!)} kg × ${reps ?? '—'}'
      : '${durationSec == null ? '—' : fmtSeconds(durationSec!)}'
          '${distanceKm == null ? '' : ', ${fmtValue(distanceKm!)} km'}';
}

/// The workout create/edit form: name/effort/notes + entry builder.
/// Returns (name, notes, effort, entries).
Future<(String, String, int?, List<WorkoutEntryInput>)?> _workoutForm(
    BuildContext context, Workout? existing) async {
  final api = AppStateScope.of(context).client!;
  final exercises = await api.listExercises();
  if (!context.mounted) return null;

  final name = TextEditingController(text: existing?.name ?? '');
  final notes = TextEditingController(text: existing?.notes ?? '');
  final effortCtrl = TextEditingController(
      text: existing?.effort == null ? '' : '${existing!.effort}');
  final entries = <_EntryDraft>[
    ...?existing?.entries.map((e) => _EntryDraft(
          exerciseId: e.exerciseId,
          exerciseName: e.exerciseName,
          exerciseType: e.exerciseType,
          weightKg: e.weightKg,
          reps: e.reps,
          durationSec: e.durationSec,
          distanceKm: e.distanceKm,
          notes: e.notes,
        )),
  ];

  final saved = await showModalBottomSheet<bool>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (sheetContext) => StatefulBuilder(
      builder: (sheetContext, setSheetState) {
        return Padding(
          padding: EdgeInsets.only(
              bottom: MediaQuery.of(sheetContext).viewInsets.bottom),
          child: SingleChildScrollView(
            child: SafeArea(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(existing == null ? 'Log workout' : 'Edit workout',
                        style: Theme.of(context).textTheme.titleLarge),
                    const SizedBox(height: 16),
                    TextField(
                      controller: name,
                      decoration: const InputDecoration(
                          labelText: 'Name (optional)',
                          border: OutlineInputBorder()),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: effortCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Effort 1-10 (optional)',
                          border: OutlineInputBorder()),
                      keyboardType: TextInputType.number,
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: notes,
                      decoration: const InputDecoration(
                          labelText: 'Notes (optional)',
                          border: OutlineInputBorder()),
                    ),
                    const SizedBox(height: 16),
                    Row(
                      children: [
                        Text('Entries',
                            style: Theme.of(context).textTheme.titleMedium),
                        const Spacer(),
                        IconButton.filledTonal(
                          tooltip: 'Add exercise',
                          onPressed: () async {
                            final draft =
                                await _pickExercise(sheetContext, exercises);
                            if (draft == null) return;
                            setSheetState(() => entries.add(draft));
                          },
                          icon: const Icon(Icons.add),
                        ),
                      ],
                    ),
                    for (final (i, e) in entries.indexed)
                      ListTile(
                        dense: true,
                        contentPadding: EdgeInsets.zero,
                        leading: Icon(e.exerciseType == 'weighted'
                            ? Icons.fitness_center
                            : Icons.timer),
                        title: Text(e.exerciseName),
                        subtitle: Text(e.summary),
                        trailing: IconButton(
                          icon: const Icon(Icons.remove_circle_outline),
                          onPressed: () =>
                              setSheetState(() => entries.removeAt(i)),
                        ),
                      ),
                    if (entries.isEmpty)
                      const Padding(
                        padding: EdgeInsets.symmetric(vertical: 8),
                        child: Text(
                          'No entries yet — add your first exercise.',
                          style: TextStyle(fontStyle: FontStyle.italic),
                        ),
                      ),
                    const SizedBox(height: 16),
                    FilledButton(
                      onPressed: () => Navigator.pop(sheetContext, true),
                      child: Text(
                          existing == null ? 'Save workout' : 'Update workout'),
                    ),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    ),
  );
  if (saved != true) return null;
  final effort = int.tryParse(effortCtrl.text.trim());
  return (
    name.text.trim(),
    notes.text.trim(),
    (effort == null || effort < 1 || effort > 10) ? null : effort,
    entries
        .map((d) => WorkoutEntryInput(
              exerciseId: d.exerciseId,
              weightKg: d.weightKg,
              reps: d.reps,
              durationSec: d.durationSec,
              distanceKm: d.distanceKm,
              notes: d.notes,
            ))
        .toList(),
  );
}

/// Exercise picker → type-aware entry editor.
Future<_EntryDraft?> _pickExercise(
    BuildContext context, List<Exercise> exercises) async {
  final selected = await showModalBottomSheet<Exercise>(
    context: context,
    showDragHandle: true,
    isScrollControlled: true,
    builder: (sheetContext) => SafeArea(
      child: ListView(
        shrinkWrap: true,
        children: [
          for (final ex in exercises)
            ListTile(
              leading: Icon(ex.isWeighted
                  ? Icons.fitness_center
                  : Icons.timer),
              title: Text(ex.name),
              subtitle: Text(ex.type),
              onTap: () => Navigator.pop(sheetContext, ex),
            ),
        ],
      ),
    ),
  );
  if (selected == null || !context.mounted) return null;
  return _entrySheet(context, selected, null);
}

/// Weighted: weight + reps. Timed: duration + optional distance.
Future<_EntryDraft?> _entrySheet(
    BuildContext context, Exercise exercise, _EntryDraft? existing) {
  final weightCtrl = TextEditingController(
      text: existing?.weightKg == null ? '' : fmtValue(existing!.weightKg!));
  final repsCtrl =
      TextEditingController(text: existing?.reps == null ? '' : '${existing!.reps}');
  final durationCtrl = TextEditingController(
      text: existing?.durationSec == null ? '' : '${existing!.durationSec}');
  final distCtrl = TextEditingController(
      text: existing?.distanceKm == null ? '' : fmtValue(existing!.distanceKm!));
  return showModalBottomSheet<_EntryDraft>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (sheetContext) => Padding(
      padding: EdgeInsets.only(
          bottom: MediaQuery.of(sheetContext).viewInsets.bottom),
      child: SingleChildScrollView(
        child: SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(exercise.name,
                    style: Theme.of(context).textTheme.titleLarge),
                const SizedBox(height: 16),
                if (exercise.isWeighted) ...[
                  Row(children: [
                    Expanded(
                        child: TextField(
                      controller: weightCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Weight (kg)',
                          border: OutlineInputBorder()),
                      keyboardType: const TextInputType.numberWithOptions(
                          decimal: true),
                      autofocus: true,
                    )),
                    const SizedBox(width: 8),
                    Expanded(
                        child: TextField(
                      controller: repsCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Reps', border: OutlineInputBorder()),
                      keyboardType: TextInputType.number,
                    )),
                  ]),
                ] else ...[
                  Row(children: [
                    Expanded(
                        child: TextField(
                      controller: durationCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Duration (min)',
                          border: OutlineInputBorder()),
                      keyboardType: const TextInputType.numberWithOptions(
                          decimal: true),
                      autofocus: true,
                    )),
                    const SizedBox(width: 8),
                    Expanded(
                        child: TextField(
                      controller: distCtrl,
                      decoration: const InputDecoration(
                          labelText: 'Distance (km, optional)',
                          border: OutlineInputBorder()),
                      keyboardType: const TextInputType.numberWithOptions(
                          decimal: true),
                    )),
                  ]),
                ],
                const SizedBox(height: 16),
                FilledButton(
                  onPressed: () {
                    if (exercise.isWeighted) {
                      final w = double.tryParse(weightCtrl.text);
                      final r = int.tryParse(repsCtrl.text);
                      if (w == null || w < 0 || r == null || r < 1) {
                        _entryError(sheetContext,
                            'Enter a non-negative weight and at least 1 rep');
                        return;
                      }
                      Navigator.pop(sheetContext, _EntryDraft(
                        exerciseId: exercise.id,
                        exerciseName: exercise.name,
                        exerciseType: exercise.type,
                        weightKg: w,
                        reps: r,
                      ));
                    } else {
                      final minutes = double.tryParse(durationCtrl.text);
                      if (minutes == null || minutes <= 0) {
                        _entryError(sheetContext, 'Enter a duration');
                        return;
                      }
                      Navigator.pop(sheetContext, _EntryDraft(
                        exerciseId: exercise.id,
                        exerciseName: exercise.name,
                        exerciseType: exercise.type,
                        durationSec: minutes * 60,
                        distanceKm: (double.tryParse(distCtrl.text)),
                      ));
                    }
                  },
                  child: const Text('Add entry'),
                ),
              ],
            ),
          ),
        ),
      ),
    ),
  );
}

void _entryError(BuildContext context, String message) {
  ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), backgroundColor: Theme.of(context).colorScheme.errorContainer));
}

/// Exercise catalog: list all, create/edit/delete custom exercises.
class ExerciseCatalogScreen extends StatefulWidget {
  const ExerciseCatalogScreen({super.key});

  @override
  State<ExerciseCatalogScreen> createState() => _ExerciseCatalogScreenState();
}

class _ExerciseCatalogScreenState extends State<ExerciseCatalogScreen> {
  int _epoch = 0;

  void _refresh() => setState(() => _epoch++);

  @override
  Widget build(BuildContext context) {
    final api = AppStateScope.of(context).client!;
    return Scaffold(
      appBar: AppBar(title: const Text('Exercises')),
      floatingActionButton: FloatingActionButton(
        tooltip: 'New exercise',
        onPressed: () async {
          final result = await _exerciseForm(context, null);
          if (result == null) return;
          try {
            await api.createExercise(
                name: result.$1,
                type: result.$2,
                muscles: result.$3,
                equipment: result.$4);
            if (context.mounted) showSnack(context, 'Exercise created');
          } on ApiException catch (e) {
            if (context.mounted) showSnack(context, e.message, error: true);
          }
          _refresh();
        },
        child: const Icon(Icons.add),
      ),
      body: LoadableList<Exercise>(
        key: ValueKey('exercises-$_epoch'),
        fetch: api.listExercises,
        emptyMessage: 'No exercises.',
        builder: (items, _) => ListView.builder(
          padding: const EdgeInsets.only(bottom: 88),
          itemCount: items.length,
          itemBuilder: (context, i) {
            final ex = items[i];
            return ListTile(
              leading: Icon(
                  ex.isWeighted ? Icons.fitness_center : Icons.timer),
              title: Text(ex.name),
              subtitle: Text(
                  '${ex.type}${ex.equipment.isNotEmpty ? ' · ${ex.equipment}' : ''}'),
              trailing: ex.isSystem
                  ? const Tooltip(
                      message: 'System exercise — immutable',
                      child: Icon(Icons.lock_outline))
                  : PopupMenuButton<String>(
                      onSelected: (action) async {
                        if (action == 'edit') {
                          final result = await _exerciseForm(context, ex);
                          if (result == null) return;
                          try {
                            await api.updateExercise(ex.id,
                                name: result.$1,
                                type: result.$2,
                                muscles: result.$3,
                                equipment: result.$4);
                            if (context.mounted) {
                              showSnack(context, 'Exercise updated');
                            }
                          } on ApiException catch (e) {
                            if (context.mounted) {
                              showSnack(context, e.message, error: true);
                            }
                          }
                        } else {
                          final ok = await showDialog<bool>(
                            context: context,
                            builder: (d) => AlertDialog(
                              title: const Text('Delete exercise?'),
                              content: Text(
                                  '${ex.name} will be removed from the catalog.'),
                              actions: [
                                TextButton(
                                    onPressed: () =>
                                        Navigator.pop(d, false),
                                    child: const Text('Cancel')),
                                FilledButton(
                                  onPressed: () => Navigator.pop(d, true),
                                  style: FilledButton.styleFrom(
                                      backgroundColor:
                                          Theme.of(d).colorScheme.error),
                                  child: const Text('Delete'),
                                ),
                              ],
                            ),
                          );
                          if (ok != true) return;
                          try {
                            await api.deleteExercise(ex.id);
                            if (context.mounted) showSnack(context, 'Deleted');
                          } on ApiException catch (e) {
                            if (context.mounted) {
                              showSnack(context, e.message, error: true);
                            }
                          }
                        }
                        _refresh();
                      },
                      itemBuilder: (_) => const [
                        PopupMenuItem(value: 'edit', child: Text('Edit')),
                        PopupMenuItem(value: 'delete', child: Text('Delete')),
                      ],
                    ),
            );
          },
        ),
      ),
    );
  }
}

/// Create/edit form; returns (name, type, muscles, equipment).
Future<(String, String, String, String)?> _exerciseForm(
    BuildContext context, Exercise? existing) {
  final name = TextEditingController(text: existing?.name ?? '');
  final muscles = TextEditingController(text: existing?.muscles ?? '');
  final equipment = TextEditingController(text: existing?.equipment ?? '');
  String type = existing?.type ?? 'weighted';
  return showDialog<(String, String, String, String)>(
    context: context,
    builder: (dialogContext) => StatefulBuilder(
      builder: (dialogContext, setDialogState) => AlertDialog(
        title: Text(existing == null ? 'New exercise' : 'Edit exercise'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
                controller: name,
                decoration: const InputDecoration(
                    labelText: 'Name', border: OutlineInputBorder()),
                autofocus: true),
            const SizedBox(height: 12),
            SegmentedButton<String>(
              segments: const [
                ButtonSegment(
                    value: 'weighted',
                    label: Text('Weighted'),
                    icon: Icon(Icons.fitness_center)),
                ButtonSegment(
                    value: 'timed',
                    label: Text('Timed'),
                    icon: Icon(Icons.timer)),
              ],
              selected: {type},
              onSelectionChanged: (s) =>
                  setDialogState(() => type = s.first),
            ),
            const SizedBox(height: 12),
            TextField(
                controller: muscles,
                decoration: const InputDecoration(
                    labelText: 'Muscles (JSON array, optional)',
                    border: OutlineInputBorder())),
            const SizedBox(height: 12),
            TextField(
                controller: equipment,
                decoration: const InputDecoration(
                    labelText: 'Equipment (optional)',
                    border: OutlineInputBorder())),
          ],
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('Cancel')),
          FilledButton(
            onPressed: () {
              if (name.text.trim().isEmpty) {
                ScaffoldMessenger.of(dialogContext).showSnackBar(
                    const SnackBar(content: Text('Name is required')));
                return;
              }
              Navigator.pop(dialogContext, (
                name.text.trim(),
                type,
                muscles.text.trim(),
                equipment.text.trim(),
              ));
            },
            child: const Text('Save'),
          ),
        ],
      ),
    ),
  );
}