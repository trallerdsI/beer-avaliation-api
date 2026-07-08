import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../entities/comment.dart';
import '../../../../core/usecase/usecase.dart';

class SendCommentParams {
  final int beerId;
  final int userId;
  final String text;
  final String userName;
  const SendCommentParams({required this.beerId, required this.userId, required this.text, required this.userName});
}

class SendCommentUseCase implements UseCase<SendCommentParams, Comment> {
  SendCommentUseCase();

  static String _stripHtml(String v) => v.replaceAll(RegExp(r'<[^>]*>'), '');

  @override
  Future<Either<Failure, Comment>> call(SendCommentParams params) async {
    final cleaned = _stripHtml(params.text).trim();
    if (cleaned.isEmpty) return Left(const Failure.unknown(message: 'Comentário vazio'));
    if (cleaned.length > 500) return Left(const Failure.unknown(message: 'Comentário muito longo'));

    final comment = Comment(
      id: DateTime.now().millisecondsSinceEpoch,
      beerId: params.beerId,
      userId: params.userId,
      text: cleaned,
      likesCount: 0,
      userName: params.userName,
      createdAt: DateTime.now(),
    );

    return Right(comment);
  }
}
