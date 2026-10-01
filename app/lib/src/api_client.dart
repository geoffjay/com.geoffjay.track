/// HTTP client for the /api/v1 JSON contract.
///
/// Authentication is a single bearer token (Authorization header) — the
/// only auth method this app supports. Lists arrive as `{"data": [...]}`;
/// single objects bare; errors as `{"error": "...", "field": "..."}` with
/// field present only on 400s. [ApiException] carries both parts so forms
/// can surface field-scoped validation messages.
library;

import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;

import 'models.dart';

/// A non-2xx API response. [status] maps the server's contract: 400
/// validation (field-scoped), 401 bad token, 403 system row, 404 missing
/// or not owned, 409 conflict.
class ApiException implements Exception {
  final int status;
  final String message;
  final String? field;

  ApiException(this.status, this.message, [this.field]);

  bool get isUnauthorized => status == 401;

  @override
  String toString() => message;
}

/// Request options shared by every call: `from`/`to` (RFC3339 or
/// YYYY-MM-DD) and `limit` (default 50, max 200).
class ListOptions {
  final DateTime? from;
  final DateTime? to;
  final int? limit;

  const ListOptions({this.from, this.to, this.limit});

  Map<String, String> toQuery({int? metricId}) => {
        if (metricId != null) 'metric_id': metricId.toString(),
        if (from != null) 'from': apiTime(from!),
        if (to != null) 'to': apiTime(to!),
        if (limit != null) 'limit': limit.toString(),
      };
}

/// Input for creating or replacing a meal's items. Either [foodId] is set
/// (nutrition computed server-side from the food's per-serving values) or
/// [label] plus explicit macros are carried (ad-hoc item).
class MealItemInput {
  final int? foodId;
  final String label;
  final double quantity;
  final double calories;
  final double protein;
  final double carbs;
  final double fat;

  const MealItemInput({
    this.foodId,
    this.label = '',
    this.quantity = 1,
    this.calories = 0,
    this.protein = 0,
    this.carbs = 0,
    this.fat = 0,
  });

  Map<String, dynamic> toJson() => {
        if (foodId != null) 'food_id': foodId,
        'label': label,
        'quantity': quantity,
        'calories': calories,
        'protein': protein,
        'carbs': carbs,
        'fat': fat,
      };
}

/// Input for creating or replacing a workout's entries. Weighted entries
/// carry weightKg + reps; timed carry durationSec (optional distanceKm).
class WorkoutEntryInput {
  final int exerciseId;
  final double? weightKg;
  final int? reps;
  final double? durationSec;
  final double? distanceKm;
  final int? effort;
  final String notes;

  const WorkoutEntryInput({
    required this.exerciseId,
    this.weightKg,
    this.reps,
    this.durationSec,
    this.distanceKm,
    this.effort,
    this.notes = '',
  });

  Map<String, dynamic> toJson() => {
        'exercise_id': exerciseId,
        if (weightKg != null) 'weight_kg': weightKg,
        if (reps != null) 'reps': reps,
        if (durationSec != null) 'duration_sec': durationSec,
        if (distanceKm != null) 'distance_km': distanceKm,
        if (effort != null) 'effort': effort,
        'notes': notes,
      };
}

class ApiClient {
  final http.Client _http;
  final String baseUrl;
  final String token;

  ApiClient({
    required this.baseUrl,
    required this.token,
    http.Client? client,
  }) : _http = client ?? http.Client();

  Map<String, String> get _headers => {
        'Authorization': 'Bearer $token',
        'Content-Type': 'application/json',
      };



  Uri _uri(String path, [Map<String, String>? query]) {
    final base = baseUrl.endsWith('/')
        ? baseUrl.substring(0, baseUrl.length - 1)
        : baseUrl;
    return Uri.parse('$base/api/v1$path').replace(queryParameters: query);
  }



  Never _fail(http.Response r, Map<String, dynamic>? body) {
    throw ApiException(
      r.statusCode,
      (body?['error'] as String?) ?? 'request failed (${r.statusCode})',
      body?['field'] as String?,
    );
  }

  Future<dynamic> _request(
    String method,
    String path, {
    Map<String, String>? query,
    Object? body,
  }) async {
    final uri = _uri(path, query);
    late http.Response r;
    try {
      final req = http.Request(method, uri)..headers.addAll(_headers);
      if (body != null) req.body = jsonEncode(body);
      final stream = await _http.send(req);
      r = await http.Response.fromStream(stream);
    } on http.ClientException {
      throw ApiException(0, 'cannot reach $baseUrl — check the address or your connection');
    }
    Map<String, dynamic>? parsed;
    if (r.body.isNotEmpty && r.body.trimLeft().startsWith('{')) {
      try {
        parsed = jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
      } on FormatException {
        parsed = null;
      }
    }
    if (r.statusCode >= 400) _fail(r, parsed);
    return parsed;
  }

  Future<List<T>> _list<T>(
    String path,
    T Function(Map<String, dynamic>) fromJson, [
    Map<String, String>? query,
  ]) async {
    final v = await _request('GET', path, query: query);
    final data = (v?['data'] as List<dynamic>? ?? []);
    return data.map((e) => fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<T> _post<T>(
    String path,
    T Function(Map<String, dynamic>) fromJson, {
    Object? body,
  }) async {
    final v = await _request('POST', path, body: body);
    return fromJson(v as Map<String, dynamic>);
  }

  Future<T> _get<T>(
    String path,
    T Function(Map<String, dynamic>) fromJson,
  ) async {
    final v = await _request('GET', path);
    return fromJson(v as Map<String, dynamic>);
  }

  Future<T> _patch<T>(
    String path,
    T Function(Map<String, dynamic>) fromJson, {
    Object? body,
  }) async {
    final v = await _request('PATCH', path, body: body);
    return fromJson(v as Map<String, dynamic>);
  }

  // ---- metrics & measurements ----

  Future<List<Metric>> listMetrics() => _list('/metrics', Metric.fromJson);

  Future<Metric> createMetric({
    required String name,
    required String unit,
    String label = '',
    String category = '',
  }) =>
      _post('/metrics', Metric.fromJson, body: {
        'name': name,
        'label': label,
        'unit': unit,
        'category': category,
      });

  Future<Metric> getMetric(int id) => _get('/metrics/$id', Metric.fromJson);

  Future<Metric> updateMetric(int id, {
    required String name,
    required String unit,
    String label = '',
    String category = '',
  }) =>
      _patch('/metrics/$id', Metric.fromJson, body: {
        'name': name,
        'label': label,
        'unit': unit,
        'category': category,
      });

  Future<void> deleteMetric(int id) async => _request('DELETE', '/metrics/$id');

  Future<List<Measurement>> listMeasurements({
    int? metricId,
    ListOptions options = const ListOptions(),
  }) =>
      _list('/measurements', Measurement.fromJson,
          options.toQuery(metricId: metricId));

  Future<Measurement> createMeasurement({
    required int metricId,
    required double value,
    DateTime? measuredAt,
    String note = '',
  }) =>
      _post('/measurements', Measurement.fromJson, body: {
        'metric_id': metricId,
        'value': value,
        if (measuredAt != null) 'measured_at': apiTime(measuredAt),
        'note': note,
      });

  /// Most recent value per metric; empty when nothing has been logged.
  Future<List<Measurement>> latestMeasurements() =>
      _list('/measurements/latest', Measurement.fromJson);

  Future<Measurement> getMeasurement(int id) =>
      _get('/measurements/$id', Measurement.fromJson);

  Future<Measurement> updateMeasurement(int id, {
    required double value,
    DateTime? measuredAt,
    String note = '',
  }) =>
      _patch('/measurements/$id', Measurement.fromJson, body: {
        'value': value,
        if (measuredAt != null) 'measured_at': apiTime(measuredAt),
        'note': note,
      });

  Future<void> deleteMeasurement(int id) async =>
      _request('DELETE', '/measurements/$id');

  // ---- foods & meals ----

  Future<List<Food>> listFoods() => _list('/foods', Food.fromJson);

  Future<Food> createFood({
    required String name,
    required double servingSize,
    required String servingUnit,
    required double calories,
    double protein = 0,
    double carbs = 0,
    double fat = 0,
    String brand = '',
  }) =>
      _post('/foods', Food.fromJson, body: {
        'name': name,
        'brand': brand,
        'serving_size': servingSize,
        'serving_unit': servingUnit,
        'calories': calories,
        'protein': protein,
        'carbs': carbs,
        'fat': fat,
      });

  Future<Food> getFood(int id) => _get('/foods/$id', Food.fromJson);

  Future<Food> updateFood(int id, {
    required String name,
    required double servingSize,
    required String servingUnit,
    required double calories,
    double protein = 0,
    double carbs = 0,
    double fat = 0,
    String brand = '',
  }) =>
      _patch('/foods/$id', Food.fromJson, body: {
        'name': name,
        'brand': brand,
        'serving_size': servingSize,
        'serving_unit': servingUnit,
        'calories': calories,
        'protein': protein,
        'carbs': carbs,
        'fat': fat,
      });

  Future<void> deleteFood(int id) async => _request('DELETE', '/foods/$id');

  Future<List<Meal>> listMeals({ListOptions options = const ListOptions()}) =>
      _list('/meals', Meal.fromJson, options.toQuery());

  Future<Meal> createMeal({
    String name = '',
    String mealType = '',
    String portion = '',
    String notes = '',
    DateTime? eatenAt,
    required List<MealItemInput> items,
  }) =>
      _post('/meals', Meal.fromJson, body: {
        'name': name,
        'meal_type': mealType,
        'portion': portion,
        'notes': notes,
        if (eatenAt != null) 'eaten_at': apiTime(eatenAt),
        'items': items.map((i) => i.toJson()).toList(),
      });

  Future<Meal> getMeal(int id) => _get('/meals/$id', Meal.fromJson);

  /// Replaces every field (the API's meal PATCH is full-replace per field
  /// group); omitting items would keep them, but the edit form always
  /// sends the full picture.
  Future<Meal> updateMeal(int id, {
    String name = '',
    String mealType = '',
    String portion = '',
    String notes = '',
    DateTime? eatenAt,
    List<MealItemInput>? items,
  }) =>
      _patch('/meals/$id', Meal.fromJson, body: {
        'name': name,
        'meal_type': mealType,
        'portion': portion,
        'notes': notes,
        if (eatenAt != null) 'eaten_at': apiTime(eatenAt),
        if (items != null) 'items': items.map((i) => i.toJson()).toList(),
      });

  Future<void> deleteMeal(int id) async => _request('DELETE', '/meals/$id');

  // ---- fasts ----

  /// The active fast, or null when none is running (the API 404s).
  Future<Fast?> activeFast() async {
    try {
      return await _get('/fasts/active', Fast.fromJson);
    } on ApiException catch (e) {
      if (e.status == 404) return null;
      rethrow;
    }
  }

  Future<List<Fast>> listFasts({ListOptions options = const ListOptions()}) =>
      _list('/fasts', Fast.fromJson, options.toQuery());

  Future<Fast> createFast({
    DateTime? startedAt,
    double? targetHours,
    String notes = '',
  }) =>
      _post('/fasts', Fast.fromJson, body: {
        if (startedAt != null) 'started_at': apiTime(startedAt),
        if (targetHours != null) 'target_hours': targetHours,
        'notes': notes,
      });

  Future<Fast> getFast(int id) => _get('/fasts/$id', Fast.fromJson);

  /// Absent fields keep their stored values (server-side merge).
  Future<Fast> updateFast(int id, {
    DateTime? startedAt,
    double? targetHours,
    String? notes,
  }) =>
      _patch('/fasts/$id', Fast.fromJson, body: {
        if (startedAt != null) 'started_at': apiTime(startedAt),
        if (targetHours != null) 'target_hours': targetHours,
        if (notes != null) 'notes': notes,
      });

  Future<Fast> endFast(int id, {DateTime? endedAt}) =>
      _post('/fasts/$id/end', Fast.fromJson,
          body: {if (endedAt != null) 'ended_at': apiTime(endedAt)});

  Future<void> deleteFast(int id) async => _request('DELETE', '/fasts/$id');

  // ---- exercises & workouts ----

  Future<List<Exercise>> listExercises() => _list('/exercises', Exercise.fromJson);

  Future<Exercise> createExercise({
    required String name,
    required String type,
    String muscles = '',
    String equipment = '',
  }) =>
      _post('/exercises', Exercise.fromJson, body: {
        'name': name,
        'type': type,
        'muscles': muscles,
        'equipment': equipment,
      });

  Future<Exercise> getExercise(int id) =>
      _get('/exercises/$id', Exercise.fromJson);

  Future<Exercise> updateExercise(int id, {
    required String name,
    required String type,
    String muscles = '',
    String equipment = '',
  }) =>
      _patch('/exercises/$id', Exercise.fromJson, body: {
        'name': name,
        'type': type,
        'muscles': muscles,
        'equipment': equipment,
      });

  Future<void> deleteExercise(int id) async =>
      _request('DELETE', '/exercises/$id');

  Future<List<Workout>> listWorkouts({
    ListOptions options = const ListOptions(),
  }) =>
      _list('/workouts', Workout.fromJson, options.toQuery());

  Future<Workout> createWorkout({
    String name = '',
    DateTime? startedAt,
    DateTime? endedAt,
    int? effort,
    String notes = '',
    List<WorkoutEntryInput>? entries,
  }) =>
      _post('/workouts', Workout.fromJson, body: {
        'name': name,
        if (startedAt != null) 'started_at': apiTime(startedAt),
        if (endedAt != null) 'ended_at': apiTime(endedAt),
        if (effort != null) 'effort': effort,
        'notes': notes,
        if (entries != null) 'entries': entries.map((e) => e.toJson()).toList(),
      });

  Future<Workout> getWorkout(int id) => _get('/workouts/$id', Workout.fromJson);

  /// Absent fields keep their stored values; entries, when present,
  /// replace all existing entries.
  Future<Workout> updateWorkout(int id, {
    String? name,
    DateTime? startedAt,
    DateTime? endedAt,
    int? effort,
    String? notes,
    List<WorkoutEntryInput>? entries,
  }) =>
      _patch('/workouts/$id', Workout.fromJson, body: {
        if (name != null) 'name': name,
        if (startedAt != null) 'started_at': apiTime(startedAt),
        if (endedAt != null) 'ended_at': apiTime(endedAt),
        if (effort != null) 'effort': effort,
        if (notes != null) 'notes': notes,
        if (entries != null) 'entries': entries.map((e) => e.toJson()).toList(),
      });

  Future<Workout> finishWorkout(int id, {DateTime? endedAt}) =>
      _post('/workouts/$id/finish', Workout.fromJson,
          body: {if (endedAt != null) 'ended_at': apiTime(endedAt)});

  Future<void> deleteWorkout(int id) async =>
      _request('DELETE', '/workouts/$id');
}