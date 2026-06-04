import 'dart:convert';
import 'package:http/http.dart' as http;

/// API client for the Go Harness HTTP server (127.0.0.1:9876).
class ApiClient {
  final String baseUrl;

  ApiClient({this.baseUrl = 'http://127.0.0.1:9876'});

  Future<Map<String, dynamic>> healthCheck() => _get('/health');

  Future<List<dynamic>> listSkills() async {
    final resp = await _get('/api/skills');
    return resp['skills'] as List<dynamic>;
  }

  Future<List<String>> listTasks() async {
    final resp = await _get('/api/tasks');
    return (resp['tasks'] as List<dynamic>).cast<String>();
  }

  Future<Map<String, dynamic>> getTask(String taskId) =>
      _get('/api/tasks/$taskId');

  Future<Map<String, dynamic>> runPipeline({
    required String skillName,
    required String trendData,
    String? taskId,
  }) =>
      _post('/api/pipeline', {
        'skill_name': skillName,
        'trend_data': trendData,
        if (taskId != null) 'task_id': taskId,
      });

  Future<Map<String, dynamic>> runStage({
    required String skillName,
    required String stage,
    required Map<String, String> input,
    String? taskId,
  }) =>
      _post('/api/stage', {
        'skill_name': skillName,
        'stage': stage,
        'input': input,
        if (taskId != null) 'task_id': taskId,
      });

  Future<Map<String, dynamic>> exportTask(String taskId, {String format = 'txt'}) =>
      _get('/api/export?task_id=$taskId&format=$format');

  Future<Map<String, dynamic>> _get(String path) async {
    final resp = await http.get(Uri.parse('$baseUrl$path'));
    if (resp.statusCode != 200) {
      throw ApiException(resp.statusCode, resp.body);
    }
    return jsonDecode(resp.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> _post(String path, Map<String, dynamic> body) async {
    final resp = await http.post(
      Uri.parse('$baseUrl$path'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode(body),
    );
    if (resp.statusCode != 200) {
      throw ApiException(resp.statusCode, resp.body);
    }
    return jsonDecode(resp.body) as Map<String, dynamic>;
  }
}

class ApiException implements Exception {
  final int statusCode;
  final String message;
  ApiException(this.statusCode, this.message);

  @override
  String toString() => 'ApiException($statusCode): $message';
}
