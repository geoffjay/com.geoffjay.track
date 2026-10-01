// End-to-end smoke: runs the real ApiClient against a locally booted
// com.geoffjay.track server. Verifies every CRUD surface the app uses.
//
// Run:  dart run test/e2e_smoke_test.dart
// Needs: TRACK_E2E_URL (default http://localhost:8199) and
//        TRACK_E2E_TOKEN (a valid bearer token) in the environment.

import 'dart:io';

import 'package:tracking/src/api_client.dart';

void check(Object cond, String what) {
  if (cond is bool && !cond) {
    stderr.writeln('FAIL: $what');
    exit(1);
  }
  stdout.writeln('ok: $what');
}

Future<void> main() async {
  final url = Platform.environment['TRACK_E2E_URL'] ?? 'http://localhost:8199';
  final token = Platform.environment['TRACK_E2E_TOKEN'];
  if (token == null || token.isEmpty) {
    stderr.writeln('set TRACK_E2E_TOKEN');
    exit(1);
  }
  final api = ApiClient(baseUrl: url, token: token);
  // Unique suffix so repeated runs don't hit UNIQUE-name conflicts with
  // rows left behind by earlier (possibly failed) runs.
  final run = DateTime.now().millisecondsSinceEpoch;
  // --- metrics catalog + measurements ---
  final metrics = await api.listMetrics();
  check(metrics.isNotEmpty, 'listMetrics returned ${metrics.length} metrics');
  final weight = metrics.firstWhere((m) => m.name == 'weight');
  final created = await api.createMetric(
      name: 'e2e_wingspan_$run', label: 'Wingspan', unit: 'cm', category: 'custom');
  check(created.id > 0 && !created.isSystem, 'createMetric -> id ${created.id}');
  final updated = await api.updateMetric(created.id,
      name: 'e2e_wingspan_$run', label: 'Wingspan (updated)', unit: 'cm');
  check(updated.label == 'Wingspan (updated)', 'updateMetric label merged');
  await api.deleteMetric(created.id);
  check(true, 'deleteMetric');

  final meas = await api.createMeasurement(
      metricId: weight.id, value: 82.5, note: 'e2e');
  check(meas.value == 82.5 && meas.metricName == 'weight',
      'createMeasurement ${meas.value}');
  final measUpd =
      await api.updateMeasurement(meas.id, value: 83.0, note: 'e2e updated');
  check(measUpd.value == 83.0, 'updateMeasurement ${measUpd.value}');
  final latest = await api.latestMeasurements();
  check(latest.any((m) => m.id == meas.id), 'latestMeasurements includes it');
  final filtered =
      await api.listMeasurements(metricId: weight.id);
  check(filtered.isNotEmpty, 'listMeasurements(metric_id) filtered');
  final got = await api.getMeasurement(meas.id);
  check(got.id == meas.id, 'getMeasurement');
  await api.deleteMeasurement(meas.id);
  check(true, 'deleteMeasurement');

  // --- foods + meals ---
  final foods = await api.listFoods();
  check(foods.isNotEmpty, 'listFoods returned ${foods.length}');
  final food = await api.createFood(
      name: 'e2e_soup_$run',
      servingSize: 250,
      servingUnit: 'ml',
      calories: 120,
      protein: 6);
  check(food.id > 0, 'createFood -> id ${food.id}');
  final foodUpd = await api.updateFood(food.id,
      name: 'e2e_soup_$run', servingSize: 250, servingUnit: 'ml', calories: 130);
  check(foodUpd.calories == 130, 'updateFood calories ${foodUpd.calories}');

  final meal = await api.createMeal(
      name: 'e2e lunch',
      mealType: 'lunch',
      portion: 'medium',
      items: [
        // quantity is in serving units: 500ml of a 250ml/130kcal food = 260.
        MealItemInput(foodId: food.id, quantity: 500),
        const MealItemInput(
            label: 'e2e side', calories: 200, protein: 10, carbs: 20, fat: 5),
      ]);
  // food 500ml at 130kcal/250ml -> 260; adhoc 200 => 460 total.
  check((meal.calories - 460).abs() < 0.01,
      'createMeal totals calories=${meal.calories} (expect 460)');
  check(meal.items.length == 2 && meal.items.first.label == 'e2e_soup_$run',
      'meal items carried through');
  final mealGot = await api.getMeal(meal.id);
  check(mealGot.items.length == 2, 'getMeal items');
  final mealUpd = await api.updateMeal(meal.id,
      name: 'e2e lunch v2',
      mealType: 'lunch',
      portion: 'large',
      items: [
        const MealItemInput(label: 'just calories', calories: 500),
      ]);
  check((mealUpd.calories - 500).abs() < 0.01,
      'updateMeal replaced items -> ${mealUpd.calories}');
  final mealsList = await api.listMeals();
  check(mealsList.any((m) => m.id == meal.id), 'listMeals');
  await api.deleteMeal(meal.id);
  check(true, 'deleteMeal');
  await api.deleteFood(food.id);
  check(true, 'deleteFood');

  // --- fasts ---
  var active = await api.activeFast();
  check(active == null, 'activeFast none initially');
  final fast = await api.createFast(targetHours: 16, notes: 'e2e fast');
  check(fast.isActive && fast.durationHours != null, 'createFast active');
  active = await api.activeFast();
  check(active?.id == fast.id, 'activeFast -> id ${active?.id}');
  // target update via PATCH (merge semantics).
  final fastUpd =
      await api.updateFast(fast.id, targetHours: 18, notes: 'e2e updated');
  check(fastUpd.targetHours == 18 && fastUpd.notes == 'e2e updated',
      'updateFast target=${fastUpd.targetHours}');
  final ended = await api.endFast(fast.id);
  check(!ended.isActive && ended.durationHours != null,
      'endFast ended (duration=${ended.durationHours}h)');

  final fasts = await api.listFasts();
  check(fasts.any((f) => f.id == fast.id), 'listFasts');
  await api.deleteFast(fast.id);
  check(true, 'deleteFast');

  // --- exercises + workouts ---
  final exercises = await api.listExercises();
  check(exercises.isNotEmpty, 'listExercises ${exercises.length}');
  final ex = await api.createExercise(
      name: 'e2e_swing_$run', type: 'weighted', equipment: 'kettlebell');
  check(ex.id > 0 && ex.isWeighted, 'createExercise -> id ${ex.id}');
  final exUpd = await api.updateExercise(ex.id,
      name: 'e2e_swing_$run', type: 'weighted', equipment: 'dumbbell');
  check(exUpd.equipment == 'dumbbell', 'updateExercise equipment');

  final bench = exercises.firstWhere((e) => e.name == 'bench_press');
  final row = exercises.firstWhere((e) => e.name == 'row');
  final workout = await api.createWorkout(
      name: 'e2e session',
      effort: 7,
      entries: [
        WorkoutEntryInput(
            exerciseId: bench.id, weightKg: 60, reps: 8, effort: 6),
        WorkoutEntryInput(exerciseId: row.id, durationSec: 1800, distanceKm: 5),
      ]);
  check(workout.entries.length == 2 && workout.name == 'e2e session',
      'createWorkout with ${workout.entries.length} entries');
  final wGot = await api.getWorkout(workout.id);
  check(wGot.entries.first.exerciseName == 'bench_press', 'getWorkout joined');
  final wUpd = await api.updateWorkout(workout.id,
      name: 'e2e session v2',
      effort: 8,
      entries: [
        WorkoutEntryInput(exerciseId: bench.id, weightKg: 62.5, reps: 6),
      ]);
  check(wUpd.entries.length == 1 && wUpd.effort == 8,
      'updateWorkout replaced entries, effort=${wUpd.effort}');
  final finished = await api.finishWorkout(workout.id);
  check(finished.isFinished && finished.durationMin! > 0,
      'finishWorkout duration=${finished.durationMin}min');
  final workouts = await api.listWorkouts();
  check(workouts.any((w) => w.id == workout.id), 'listWorkouts');
  await api.deleteWorkout(workout.id);
  check(true, 'deleteWorkout');
  await api.deleteExercise(ex.id);
  check(true, 'deleteExercise');

  // --- error contract ---
  try {
    await api.getMeasurement(999999);
    stderr.writeln('FAIL: expected 404');
    exit(1);
  } on ApiException catch (e) {
    check(e.status == 404, '404 -> ApiException status ${e.status}');
  }
  try {
    await api.createMeasurement(metricId: 999999, value: 1);
    stderr.writeln('FAIL: expected 400');
    exit(1);
  } on ApiException catch (e) {
    check(e.status == 400 && e.field == 'metric_id',
        '400 field-scoped -> ${e.status} ${e.field}');
  }

  stdout.writeln('\nE2E OK — all CRUD surfaces verified against $url');
}